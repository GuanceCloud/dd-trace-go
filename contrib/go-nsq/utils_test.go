package nsq

import (
	"testing"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/mocktracer"
	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/stretchr/testify/assert"
)

func TestInject(t *testing.T) {
	mt := mocktracer.Start()
	defer mt.Stop()

	span := tracer.StartSpan("test.go-nsq.utils")
	defer span.Finish()

	body := []byte("test data")
	injectedBody, err := inject(span, body)
	if err != nil {
		t.Fatal(err.Error())
	}

	spnctx, newbody, err := extract(injectedBody)
	if err != nil {
		t.Fatal(err.Error())
	}

	assert.Equal(t, span.Context().TraceID(), spnctx.TraceID())
	assert.Equal(t, span.Context().SpanID(), spnctx.SpanID())
	assert.Equal(t, newbody, body)
}
