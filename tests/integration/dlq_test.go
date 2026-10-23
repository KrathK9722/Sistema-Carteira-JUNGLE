//go:build integration

package integration

import (
	"context"
	"example.com/financial-service/internal/configuration"
	"example.com/financial-service/internal/domain"
	"example.com/financial-service/internal/infrastructure/messaging"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"testing"
	"time"
)

func TestPoisonMessageReachesDLQ(t *testing.T) {
	waitReady(t)
	queues := messaging.NewClient(configuration.Config{AWSRegion: "us-east-1", AWSKey: "test", AWSSecret: "test", SQSEndpoint: "http://localhost:4566"})
	err := queues.Ensure(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	id, err := domain.NewID()
	if err != nil {
		t.Fatal(err)
	}
	body := "invalid-json-" + id
	_, err = queues.API.SendMessage(context.Background(), &sqs.SendMessageInput{QueueUrl: aws.String(queues.QueueURL), MessageBody: aws.String(body), MessageGroupId: aws.String(id), MessageDeduplicationId: aws.String(id)})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(150 * time.Second)
	for time.Now().Before(deadline) {
		batch, receiveErr := queues.API.ReceiveMessage(context.Background(), &sqs.ReceiveMessageInput{QueueUrl: aws.String(queues.DLQURL), MaxNumberOfMessages: 10, WaitTimeSeconds: 2, VisibilityTimeout: 5})
		if receiveErr != nil {
			t.Fatal(receiveErr)
		}
		for _, message := range batch.Messages {
			if aws.ToString(message.Body) == body {
				_, err = queues.API.DeleteMessage(context.Background(), &sqs.DeleteMessageInput{QueueUrl: aws.String(queues.DLQURL), ReceiptHandle: message.ReceiptHandle})
				if err != nil {
					t.Fatal(err)
				}
				t.Log("invalid message was retried and redriven to the real FIFO DLQ")
				return
			}
		}
	}
	t.Fatal("poison message never reached the DLQ")
}
