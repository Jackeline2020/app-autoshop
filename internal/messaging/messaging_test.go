package messaging_test

import (
	"context"
	"testing"

	"autoshop/internal/messaging"
	"autoshop/internal/mocks"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMessageAndDecode(t *testing.T) {
	env, err := messaging.NewMessage(messaging.CmdReserveParts, "saga-1", "os-1", "corr-1",
		messaging.ReservePartsPayload{Items: []messaging.ReservationItem{{PartID: "p1", Quantity: 2}}})
	require.NoError(t, err)
	assert.NotEmpty(t, env.MessageID)
	assert.NotEmpty(t, env.OccurredAt)

	var p messaging.ReservePartsPayload
	require.NoError(t, env.DecodePayload(&p))
	assert.Equal(t, 2, p.Items[0].Quantity)
}

func TestMessageWithoutPayload(t *testing.T) {
	env, err := messaging.NewMessage(messaging.CmdStartDiagnosis, "s", "o", "c", nil)
	require.NoError(t, err)
	assert.ErrorIs(t, env.DecodePayload(&struct{}{}), messaging.ErrEmptyPayload)

	_, err = messaging.NewMessage("x", "s", "o", "c", make(chan int))
	assert.Error(t, err)
}

func TestSender(t *testing.T) {
	fake := &mocks.SQSFake{}
	sender := messaging.NewSender(fake, map[string]string{messaging.QueueBilling: "http://queue/billing-commands"})

	body := []byte(`{"message_id":"m1","type":"GenerateBudget","correlation_id":"corr-1"}`)
	require.NoError(t, sender.Send(context.Background(), messaging.QueueBilling, body))
	require.Len(t, fake.Sent, 1)
	assert.Equal(t, "http://queue/billing-commands", aws.ToString(fake.Sent[0].QueueUrl))
	assert.Equal(t, "GenerateBudget", aws.ToString(fake.Sent[0].MessageAttributes["type"].StringValue))
	assert.Equal(t, "corr-1", aws.ToString(fake.Sent[0].MessageAttributes["correlation_id"].StringValue))

	assert.Error(t, sender.Send(context.Background(), "fila-inexistente", body))

	require.NoError(t, sender.Send(context.Background(), messaging.QueueBilling, []byte(`{}`)))
	assert.Empty(t, fake.Sent[1].MessageAttributes)
}

func TestQueueURL(t *testing.T) {
	fake := &mocks.SQSFake{QueueURLs: map[string]string{"os-events": "http://queue/os-events"}}
	url, err := messaging.QueueURL(context.Background(), fake, "os-events")
	require.NoError(t, err)
	assert.Equal(t, "http://queue/os-events", url)
	_, err = messaging.QueueURL(context.Background(), fake, "x")
	assert.Error(t, err)
}
