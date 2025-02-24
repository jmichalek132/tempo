package combiner

import (
	"fmt"

	"github.com/grafana/tempo/pkg/model/trace"
	"github.com/grafana/tempo/pkg/tempopb"
)

func NewTypedTraceByIDV2(maxBytes int, marshalingFormat string) GRPCCombiner[*tempopb.TraceByIDResponse] {
	return NewTraceByIDV2(maxBytes, marshalingFormat).(GRPCCombiner[*tempopb.TraceByIDResponse])
}

func NewTraceByIDV2(maxBytes int, marshalingFormat string) Combiner {
	combiner := trace.NewCombiner(maxBytes, true)
	var partialTrace bool
	var inspectedBytes uint64
	gc := &genericCombiner[*tempopb.TraceByIDResponse]{
		combine: func(partial *tempopb.TraceByIDResponse, _ *tempopb.TraceByIDResponse, _ PipelineResponse) error {
			if partial.Status == tempopb.TraceByIDResponse_PARTIAL {
				partialTrace = true
			}
			if partial.Metrics != nil {
				inspectedBytes += partial.Metrics.InspectedBytes
			}
			_, err := combiner.Consume(partial.Trace)
			return err
		},
		finalize: func(resp *tempopb.TraceByIDResponse) (*tempopb.TraceByIDResponse, error) {
			traceResult, _ := combiner.Result()
			if traceResult == nil {
				traceResult = &tempopb.Trace{}
			}

			// dedupe duplicate span ids
			deduper := newDeduper()
			traceResult = deduper.dedupe(traceResult)
			resp.Trace = traceResult
			resp.Metrics = &tempopb.TraceByIDMetrics{InspectedBytes: inspectedBytes}

			if partialTrace || combiner.IsPartialTrace() {
				resp.Status = tempopb.TraceByIDResponse_PARTIAL
				resp.Message = fmt.Sprintf("Trace exceeds maximum size of %d bytes, a partial trace is returned", maxBytes)
			}

			return resp, nil
		},
		new:     func() *tempopb.TraceByIDResponse { return &tempopb.TraceByIDResponse{} },
		current: &tempopb.TraceByIDResponse{},
	}
	initHTTPCombiner(gc, marshalingFormat)
	return gc
}
