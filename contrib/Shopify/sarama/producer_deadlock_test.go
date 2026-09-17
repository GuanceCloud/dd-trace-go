// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016 Datadog, Inc.

package sarama

import (
	"sync"
	"testing"
	"time"

	"github.com/GuanceCloud/dd-trace-go/v2/ddtrace/mocktracer"
	"github.com/Shopify/sarama"
	"github.com/stretchr/testify/require"
)

func TestWrapAsyncProducerDrainsResultsWhileInputBlocked(t *testing.T) {
	mt := mocktracer.Start()
	defer mt.Stop()

	producer := newBlockingAsyncProducer()
	config := sarama.NewConfig()
	config.Version = sarama.V0_11_0_0
	config.Producer.Return.Successes = true
	wrapper := WrapAsyncProducer(config, producer)

	first := &sarama.ProducerMessage{Topic: "first"}
	second := &sarama.ProducerMessage{Topic: "second"}
	releaseSuccess := make(chan struct{})
	firstReceived := make(chan struct{})
	secondReceived := make(chan struct{})
	go func() {
		msg := <-producer.input
		close(firstReceived)
		<-releaseSuccess
		producer.successes <- msg
		<-producer.input
		close(secondReceived)
	}()

	sendWithTimeout(t, wrapper.Input(), first)
	waitWithTimeout(t, firstReceived, "underlying producer did not receive the first message")
	sendWithTimeout(t, wrapper.Input(), second)
	close(releaseSuccess)

	select {
	case got := <-wrapper.Successes():
		require.Same(t, first, got)
	case <-time.After(time.Second):
		t.Fatal("wrapper did not drain the success while the underlying input was blocked")
	}
	waitWithTimeout(t, secondReceived, "underlying producer did not receive the second message")
	require.NoError(t, wrapper.Close())
}

func sendWithTimeout(t *testing.T, input chan<- *sarama.ProducerMessage, msg *sarama.ProducerMessage) {
	t.Helper()
	select {
	case input <- msg:
	case <-time.After(time.Second):
		t.Fatalf("wrapper did not accept message for topic %q", msg.Topic)
	}
}

func waitWithTimeout(t *testing.T, done <-chan struct{}, message string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal(message)
	}
}

type blockingAsyncProducer struct {
	input     chan *sarama.ProducerMessage
	successes chan *sarama.ProducerMessage
	errors    chan *sarama.ProducerError
	closeOnce sync.Once
}

func newBlockingAsyncProducer() *blockingAsyncProducer {
	return &blockingAsyncProducer{
		input:     make(chan *sarama.ProducerMessage),
		successes: make(chan *sarama.ProducerMessage),
		errors:    make(chan *sarama.ProducerError),
	}
}

func (p *blockingAsyncProducer) AsyncClose() { _ = p.Close() }

func (p *blockingAsyncProducer) Close() error {
	p.closeOnce.Do(func() {
		close(p.successes)
		close(p.errors)
	})
	return nil
}

func (p *blockingAsyncProducer) Input() chan<- *sarama.ProducerMessage { return p.input }

func (p *blockingAsyncProducer) Successes() <-chan *sarama.ProducerMessage { return p.successes }

func (p *blockingAsyncProducer) Errors() <-chan *sarama.ProducerError { return p.errors }

func (p *blockingAsyncProducer) IsTransactional() bool { return false }

func (p *blockingAsyncProducer) TxnStatus() sarama.ProducerTxnStatusFlag { return 0 }

func (p *blockingAsyncProducer) BeginTxn() error { return nil }

func (p *blockingAsyncProducer) CommitTxn() error { return nil }

func (p *blockingAsyncProducer) AbortTxn() error { return nil }

func (p *blockingAsyncProducer) AddOffsetsToTxn(map[string][]*sarama.PartitionOffsetMetadata, string) error {
	return nil
}

func (p *blockingAsyncProducer) AddMessageToTxn(*sarama.ConsumerMessage, string, *string) error {
	return nil
}
