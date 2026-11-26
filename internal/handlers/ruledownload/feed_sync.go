package ruledownload

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/airbnb/rudolph/pkg/dynamodb"
	"github.com/airbnb/rudolph/pkg/model/feedrules"
	"github.com/airbnb/rudolph/pkg/model/rules"
	"github.com/airbnb/rudolph/pkg/response"
	"github.com/aws/aws-lambda-go/events"
)

type feedRuleDownloader interface {
	handle(machineID string, cursor ruledownloadCursor) (*events.APIGatewayProxyResponse, error)
}

type concreteFeedRuleDownloader struct {
	queryer dynamodb.QueryAPI
}

func (d concreteFeedRuleDownloader) handle(machineID string, cursor ruledownloadCursor) (*events.APIGatewayProxyResponse, error) {
	ddbCursor := cursor.GetLastEvaluatedKey()

	feedRules, lastEvaluatedKey, err := feedrules.GetPaginatedFeedRules(d.queryer, cursor.BatchSize, ddbCursor)
	if err != nil {
		slog.Error("Failed to query feed rules")
		return response.APIResponse(http.StatusInternalServerError, response.ErrInternalServerErrorResponse)
	}

	nextCursor := cursor.CloneForNextPage()
	if lastEvaluatedKey == nil {
		nextCursor.SetStrategy(ruledownloadStrategyMachine)
	} else {
		// Here we inherit the preexisting cursor strategy
		nextCursor.SetDynamodbLastEvaluatedKey(lastEvaluatedKey)
	}

	rules := make([]rules.SantaRule, len(feedRules))
	for i, rule := range feedRules {
		rules[i] = rule.SantaRule
	}

	// Marshal the cursor to a string
	jsonCursor, err := json.Marshal(nextCursor)
	if err != nil {
		slog.Error("Failed to serialize rule cursor")
		return response.APIResponse(http.StatusInternalServerError, response.ErrInternalServerErrorResponse)
	}

	return response.APIResponse(
		http.StatusOK,
		RuledownloadResponse{
			Rules:  DDBRulesToResponseRules(rules),
			Cursor: string(jsonCursor),
		},
	)
}
