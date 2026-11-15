package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// SQSAPI reúne as operações do SQS usadas pelo serviço (permite testar
// sem a AWS).
type SQSAPI interface {
	SendMessage(ctx context.Context, in *sqs.SendMessageInput, opts ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
	ReceiveMessage(ctx context.Context, in *sqs.ReceiveMessageInput, opts ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, in *sqs.DeleteMessageInput, opts ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
	GetQueueUrl(ctx context.Context, in *sqs.GetQueueUrlInput, opts ...func(*sqs.Options)) (*sqs.GetQueueUrlOutput, error)
}

// Sender publica mensagens já serializadas nas filas de destino.
type Sender struct {
	client SQSAPI
	queues map[string]string // nome lógico → URL
}

func NewSender(client SQSAPI, queues map[string]string) *Sender {
	return &Sender{client: client, queues: queues}
}

// Send publica o corpo na fila indicada, com o tipo e o correlation_id
// como atributos da mensagem.
func (s *Sender) Send(ctx context.Context, queue string, body []byte) error {
	url, ok := s.queues[queue]
	if !ok {
		return fmt.Errorf("fila desconhecida: %s", queue)
	}

	var env Envelope
	_ = json.Unmarshal(body, &env)
	attrs := map[string]types.MessageAttributeValue{}
	if env.Type != "" {
		attrs["type"] = types.MessageAttributeValue{DataType: aws.String("String"), StringValue: aws.String(env.Type)}
	}
	if env.CorrelationID != "" {
		attrs["correlation_id"] = types.MessageAttributeValue{DataType: aws.String("String"), StringValue: aws.String(env.CorrelationID)}
	}

	_, err := s.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:          aws.String(url),
		MessageBody:       aws.String(string(body)),
		MessageAttributes: attrs,
	})
	return err
}

// QueueURL descobre a URL de uma fila pelo nome.
func QueueURL(ctx context.Context, client SQSAPI, name string) (string, error) {
	out, err := client.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{QueueName: aws.String(name)})
	if err != nil {
		return "", fmt.Errorf("fila %s não encontrada: %w", name, err)
	}
	return aws.ToString(out.QueueUrl), nil
}
