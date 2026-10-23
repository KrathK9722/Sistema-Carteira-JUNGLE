package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"example.com/financial-service/internal/configuration"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type Client struct {
	API                 *sqs.Client
	QueueURL, EventsURL string
}

func NewClient(config configuration.Config) *Client {
	sdkConfig := aws.Config{Region: config.AWSRegion, Credentials: credentials.NewStaticCredentialsProvider(config.AWSKey, config.AWSSecret, "")}
	options := sqs.Options{Region: sdkConfig.Region, Credentials: sdkConfig.Credentials, BaseEndpoint: aws.String(config.SQSEndpoint)}
	return &Client{API: sqs.New(options), QueueURL: config.QueueURL, EventsURL: config.EventsURL}
}
func (client *Client) Ensure(ctx context.Context) error {
	dlq, err := client.API.CreateQueue(ctx, &sqs.CreateQueueInput{QueueName: aws.String("wager-transactions-dlq.fifo"), Attributes: map[string]string{"FifoQueue": "true"}})
	if err != nil {
		return err
	}
	attrs, err := client.API.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{QueueUrl: dlq.QueueUrl, AttributeNames: []types.QueueAttributeName{types.QueueAttributeNameQueueArn}})
	if err != nil {
		return err
	}
	policy, err := json.Marshal(map[string]string{"deadLetterTargetArn": attrs.Attributes["QueueArn"], "maxReceiveCount": "5"})
	if err != nil {
		return err
	}
	queue, err := client.API.CreateQueue(ctx, &sqs.CreateQueueInput{QueueName: aws.String("wager-transactions.fifo"), Attributes: map[string]string{"FifoQueue": "true", "VisibilityTimeout": "60", "ReceiveMessageWaitTimeSeconds": "10", "RedrivePolicy": string(policy)}})
	if err != nil {
		return err
	}
	events, err := client.API.CreateQueue(ctx, &sqs.CreateQueueInput{QueueName: aws.String("wallet-events.fifo"), Attributes: map[string]string{"FifoQueue": "true"}})
	if err != nil {
		return err
	}
	if client.QueueURL == "" {
		client.QueueURL = aws.ToString(queue.QueueUrl)
	}
	if client.EventsURL == "" {
		client.EventsURL = aws.ToString(events.QueueUrl)
	}
	return nil
}
func (client *Client) Ready(ctx context.Context) error {
	if client.QueueURL == "" || client.EventsURL == "" {
		return errors.New("queues not initialized")
	}
	for _, url := range []string{client.QueueURL, client.EventsURL} {
		_, err := client.API.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{QueueUrl: aws.String(url)})
		if err != nil {
			return err
		}
	}
	return nil
}
func (client *Client) Publish(ctx context.Context, id, aggregate, payload string) error {
	_, err := client.API.SendMessage(ctx, &sqs.SendMessageInput{QueueUrl: aws.String(client.EventsURL), MessageBody: aws.String(payload), MessageGroupId: aws.String(aggregate), MessageDeduplicationId: aws.String(id)})
	if err != nil {
		return fmt.Errorf("publish outbox event: %w", err)
	}
	return nil
}
