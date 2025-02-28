// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package solacereceiver

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Azure/go-amqp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	model_v1 "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver/model/v1"
)

// Validate entire unmarshal flow
func TestSolaceMessageUnmarshallerUnmarshal(t *testing.T) {
	validTopicVersion := "_telemetry/broker/trace/receive/v1"
	invalidTopicVersion := "_telemetry/broker/trace/receive/v2"
	invalidTopicString := "some unknown topic string that won't be valid"

	tests := []struct {
		name    string
		message *amqp.Message
		want    *ptrace.Traces
		err     error
	}{
		{
			name: "Unknown Topic Stirng",
			message: &inboundMessage{
				Properties: &amqp.MessageProperties{
					To: &invalidTopicString,
				},
			},
			err: errUnknownTraceMessgeType,
		},
		{
			name: "Bad Topic Version",
			message: &inboundMessage{
				Properties: &amqp.MessageProperties{
					To: &invalidTopicVersion,
				},
			},
			err: errUnknownTraceMessgeVersion,
		},
		{
			name: "No Message Properties",
			message: &inboundMessage{
				Properties: nil,
			},
			err: errUnknownTraceMessgeType,
		},
		{
			name: "No Topic String",
			message: &inboundMessage{
				Properties: &amqp.MessageProperties{
					To: nil,
				},
			},
			err: errUnknownTraceMessgeType,
		},
		{
			name: "Empty Message Data",
			message: &amqp.Message{
				Data: [][]byte{{}},
				Properties: &amqp.MessageProperties{
					To: &validTopicVersion,
				},
			},
			err: errEmptyPayload,
		},
		{
			name: "Invalid Message Data",
			message: &amqp.Message{
				Data: [][]byte{{1, 2, 3, 4, 5}},
				Properties: &amqp.MessageProperties{
					To: &validTopicVersion,
				},
			},
			err: errors.New("cannot parse invalid wire-format data"),
		},
		{
			name: "Valid Message Data",
			message: &amqp.Message{
				Data: [][]byte{func() []byte {
					// TODO capture binary data of this directly, ie. real world data.
					var (
						protocolVersion      = "5.0"
						applicationMessageID = "someMessageID"
						correlationID        = "someConversationID"
						priority             = uint32(1)
						ttl                  = int64(86000)
						routerName           = "someRouterName"
						vpnName              = "someVpnName"
						replyToTopic         = "someReplyToTopic"
						topic                = "someTopic"
					)
					validData, err := proto.Marshal(&model_v1.SpanData{
						TraceId:                             []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
						SpanId:                              []byte{7, 6, 5, 4, 3, 2, 1, 0},
						StartTimeUnixNano:                   1234567890,
						EndTimeUnixNano:                     2234567890,
						RouterName:                          routerName,
						MessageVpnName:                      &vpnName,
						SolosVersion:                        "10.0.0",
						Protocol:                            "MQTT",
						ProtocolVersion:                     &protocolVersion,
						ApplicationMessageId:                &applicationMessageID,
						CorrelationId:                       &correlationID,
						DeliveryMode:                        model_v1.SpanData_DIRECT,
						BinaryAttachmentSize:                1000,
						XmlAttachmentSize:                   200,
						MetadataSize:                        34,
						ClientUsername:                      "someClientUsername",
						ClientName:                          "someClient1234",
						Topic:                               topic,
						ReplyToTopic:                        &replyToTopic,
						ReplicationGroupMessageId:           []byte{0x01, 0x00, 0x01, 0x04, 0x09, 0x10, 0x19, 0x24, 0x31, 0x40, 0x51, 0x64, 0x79, 0x90, 0xa9, 0xc4, 0xe1},
						Priority:                            &priority,
						Ttl:                                 &ttl,
						DmqEligible:                         true,
						DroppedEnqueueEventsSuccess:         42,
						DroppedEnqueueEventsFailed:          24,
						HostIp:                              []byte{1, 2, 3, 4},
						HostPort:                            55555,
						PeerIp:                              []byte{35, 69, 4, 37, 44, 161, 0, 0, 0, 0, 5, 103, 86, 115, 35, 181},
						PeerPort:                            12345,
						BrokerReceiveTimeUnixNano:           1357924680,
						DroppedApplicationMessageProperties: false,
						UserProperties: map[string]*model_v1.SpanData_UserPropertyValue{
							"special_key": {
								Value: &model_v1.SpanData_UserPropertyValue_BoolValue{
									BoolValue: true,
								},
							},
						},
						EnqueueEvents: []*model_v1.SpanData_EnqueueEvent{
							{
								Dest:         &model_v1.SpanData_EnqueueEvent_QueueName{QueueName: "somequeue"},
								TimeUnixNano: 123456789,
							},
							{
								Dest:         &model_v1.SpanData_EnqueueEvent_TopicEndpointName{TopicEndpointName: "sometopic"},
								TimeUnixNano: 2345678,
							},
						},
						TransactionEvent: &model_v1.SpanData_TransactionEvent{
							TimeUnixNano: 123456789,
							Type:         model_v1.SpanData_TransactionEvent_SESSION_TIMEOUT,
							Initiator:    model_v1.SpanData_TransactionEvent_CLIENT,
							TransactionId: &model_v1.SpanData_TransactionEvent_LocalId{
								LocalId: &model_v1.SpanData_TransactionEvent_LocalTransactionId{
									TransactionId: 12345,
									SessionId:     67890,
									SessionName:   "my-session-name",
								},
							},
						},
					})
					require.NoError(t, err)
					return validData
				}()},
				Properties: &amqp.MessageProperties{
					To: &validTopicVersion,
				},
			},
			want: func() *ptrace.Traces {
				traces := ptrace.NewTraces()
				resource := traces.ResourceSpans().AppendEmpty()
				populateAttributes(t, resource.Resource().Attributes(), map[string]interface{}{
					"service.name":        "someRouterName",
					"service.instance.id": "someVpnName",
					"service.version":     "10.0.0",
				})
				instrumentation := resource.ScopeSpans().AppendEmpty()
				span := instrumentation.Spans().AppendEmpty()
				span.SetTraceID([16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15})
				span.SetSpanID([8]byte{7, 6, 5, 4, 3, 2, 1, 0})
				span.SetStartTimestamp(1234567890)
				span.SetEndTimestamp(2234567890)
				// expect some constants
				span.SetKind(5)
				span.SetName("(topic) receive")
				span.Status().SetCode(ptrace.StatusCodeUnset)
				spanAttrs := span.Attributes()
				populateAttributes(t, spanAttrs, map[string]interface{}{
					"messaging.system":                                        "SolacePubSub+",
					"messaging.operation":                                     "receive",
					"messaging.protocol":                                      "MQTT",
					"messaging.protocol_version":                              "5.0",
					"messaging.message_id":                                    "someMessageID",
					"messaging.conversation_id":                               "someConversationID",
					"messaging.message_payload_size_bytes":                    int64(1234),
					"messaging.destination":                                   "someTopic",
					"messaging.solace.client_username":                        "someClientUsername",
					"messaging.solace.client_name":                            "someClient1234",
					"messaging.solace.replication_group_message_id":           "rmid1:00010-40910192431-40516479-90a9c4e1",
					"messaging.solace.priority":                               int64(1),
					"messaging.solace.ttl":                                    int64(86000),
					"messaging.solace.dmq_eligible":                           true,
					"messaging.solace.dropped_enqueue_events_success":         int64(42),
					"messaging.solace.dropped_enqueue_events_failed":          int64(24),
					"messaging.solace.reply_to_topic":                         "someReplyToTopic",
					"messaging.solace.broker_receive_time_unix_nano":          int64(1357924680),
					"messaging.solace.dropped_application_message_properties": false,
					"messaging.solace.delivery_mode":                          "direct",
					"net.host.ip":                                             "1.2.3.4",
					"net.host.port":                                           int64(55555),
					"net.peer.ip":                                             "2345:425:2ca1::567:5673:23b5",
					"net.peer.port":                                           int64(12345),
					"messaging.solace.user_properties.special_key":            true,
				})
				populateEvent(t, span, "somequeue enqueue", 123456789, map[string]interface{}{
					"messaging.solace.destination_type":     "queue",
					"messaging.solace.rejects_all_enqueues": false,
				})
				populateEvent(t, span, "sometopic enqueue", 2345678, map[string]interface{}{
					"messaging.solace.destination_type":     "topic-endpoint",
					"messaging.solace.rejects_all_enqueues": false,
				})
				populateEvent(t, span, "session_timeout", 123456789, map[string]interface{}{
					"messaging.solace.transaction_initiator":   "client",
					"messaging.solace.transaction_id":          12345,
					"messaging.solace.transacted_session_name": "my-session-name",
					"messaging.solace.transacted_session_id":   67890,
				})
				return &traces
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newTracesUnmarshaller(zap.NewNop(), newTestMetrics(t))
			traces, err := u.unmarshal(tt.message)
			if tt.err != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.err.Error())
			} else {
				assert.NoError(t, err)
			}

			if tt.want != nil {
				require.NotNil(t, traces)
				require.Equal(t, 1, traces.ResourceSpans().Len())
				expectedResource := tt.want.ResourceSpans().At(0)
				resource := traces.ResourceSpans().At(0)
				assert.Equal(t, expectedResource.Resource().Attributes().AsRaw(), resource.Resource().Attributes().AsRaw())
				require.Equal(t, 1, resource.ScopeSpans().Len())
				expectedInstrumentation := expectedResource.ScopeSpans().At(0)
				instrumentation := resource.ScopeSpans().At(0)
				assert.Equal(t, expectedInstrumentation.Scope(), instrumentation.Scope())
				require.Equal(t, 1, instrumentation.Spans().Len())
				expectedSpan := expectedInstrumentation.Spans().At(0)
				span := instrumentation.Spans().At(0)
				compareSpans(t, expectedSpan, span)
			} else {
				assert.Equal(t, ptrace.Traces{}, traces)
			}
		})
	}
}

func TestUnmarshallerMapResourceSpan(t *testing.T) {
	var (
		routerName = "someRouterName"
		vpnName    = "someVpnName"
		version    = "10.0.0"
	)
	tests := []struct {
		name                        string
		spanData                    *model_v1.SpanData
		want                        map[string]interface{}
		expectedUnmarshallingErrors interface{}
	}{
		{
			name: "Maps All Fields When Present",
			spanData: &model_v1.SpanData{
				RouterName:     routerName,
				MessageVpnName: &vpnName,
				SolosVersion:   version,
			},
			want: map[string]interface{}{
				"service.name":        routerName,
				"service.instance.id": vpnName,
				"service.version":     version,
			},
		},
		{
			name:     "Does Not Map Fields When Not Present",
			spanData: &model_v1.SpanData{},
			want: map[string]interface{}{
				"service.version": "",
				"service.name":    "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newTestV1Unmarshaller(t)
			actual := pcommon.NewMap()
			u.mapResourceSpanAttributes(tt.spanData, actual)
			assert.Equal(t, tt.want, actual.AsRaw())
			validateMetric(t, u.metrics.views.recoverableUnmarshallingErrors, tt.expectedUnmarshallingErrors)
		})
	}
}

// Tests the received span to traces mappings
// Includes all required opentelemetry fields such as trace ID, span ID, etc.
func TestUnmarshallerMapClientSpanData(t *testing.T) {
	someTraceState := "some trace status"
	tests := []struct {
		name string
		data *model_v1.SpanData
		want func(ptrace.Span)
	}{
		// no trace state no status no parent span
		{
			name: "Without Optional Fields",
			data: &model_v1.SpanData{
				TraceId:           []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
				SpanId:            []byte{7, 6, 5, 4, 3, 2, 1, 0},
				StartTimeUnixNano: 1234567890,
				EndTimeUnixNano:   2234567890,
			},
			want: func(span ptrace.Span) {
				span.SetTraceID([16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15})
				span.SetSpanID([8]byte{7, 6, 5, 4, 3, 2, 1, 0})
				span.SetStartTimestamp(1234567890)
				span.SetEndTimestamp(2234567890)
				// expect some constants
				span.SetKind(5)
				span.SetName("(topic) receive")
				span.Status().SetCode(ptrace.StatusCodeUnset)
			},
		},
		// trace state status and parent span
		{
			name: "With Optional Fields",
			data: &model_v1.SpanData{
				TraceId:           []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
				SpanId:            []byte{7, 6, 5, 4, 3, 2, 1, 0},
				StartTimeUnixNano: 1234567890,
				EndTimeUnixNano:   2234567890,
				ParentSpanId:      []byte{15, 14, 13, 12, 11, 10, 9, 8},
				TraceState:        &someTraceState,
				ErrorDescription:  "some error",
			},
			want: func(span ptrace.Span) {
				span.SetTraceID([16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15})
				span.SetSpanID([8]byte{7, 6, 5, 4, 3, 2, 1, 0})
				span.SetStartTimestamp(1234567890)
				span.SetEndTimestamp(2234567890)
				span.SetParentSpanID([8]byte{15, 14, 13, 12, 11, 10, 9, 8})
				span.TraceState().FromRaw(someTraceState)
				span.Status().SetCode(ptrace.StatusCodeError)
				span.Status().SetMessage("some error")
				// expect some constants
				span.SetKind(5)
				span.SetName("(topic) receive")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newTestV1Unmarshaller(t)
			actual := ptrace.NewTraces().ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
			u.mapClientSpanData(tt.data, actual)
			expected := ptrace.NewTraces().ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
			tt.want(expected)
			assert.Equal(t, expected, actual)
		})
	}
}

func TestUnmarshallerMapClientSpanAttributes(t *testing.T) {
	var (
		protocolVersion      = "5.0"
		applicationMessageID = "someMessageID"
		correlationID        = "someConversationID"
		replyToTopic         = "someReplyToTopic"
		priority             = uint32(1)
		ttl                  = int64(86000)
	)

	tests := []struct {
		name                        string
		spanData                    *model_v1.SpanData
		want                        map[string]interface{}
		expectedUnmarshallingErrors interface{}
	}{
		{
			name: "With All Valid Attributes",
			spanData: &model_v1.SpanData{
				Protocol:                            "MQTT",
				ProtocolVersion:                     &protocolVersion,
				ApplicationMessageId:                &applicationMessageID,
				CorrelationId:                       &correlationID,
				BinaryAttachmentSize:                1000,
				XmlAttachmentSize:                   200,
				MetadataSize:                        34,
				ClientUsername:                      "someClientUsername",
				ClientName:                          "someClient1234",
				ReplyToTopic:                        &replyToTopic,
				DeliveryMode:                        model_v1.SpanData_PERSISTENT,
				Topic:                               "someTopic",
				ReplicationGroupMessageId:           []byte{0x01, 0x00, 0x01, 0x04, 0x09, 0x10, 0x19, 0x24, 0x31, 0x40, 0x51, 0x64, 0x79, 0x90, 0xa9, 0xc4, 0xe1},
				Priority:                            &priority,
				Ttl:                                 &ttl,
				DmqEligible:                         true,
				DroppedEnqueueEventsSuccess:         42,
				DroppedEnqueueEventsFailed:          24,
				HostIp:                              []byte{1, 2, 3, 4},
				HostPort:                            55555,
				PeerIp:                              []byte{35, 69, 4, 37, 44, 161, 0, 0, 0, 0, 5, 103, 86, 115, 35, 181},
				PeerPort:                            12345,
				BrokerReceiveTimeUnixNano:           1357924680,
				DroppedApplicationMessageProperties: false,
				UserProperties: map[string]*model_v1.SpanData_UserPropertyValue{
					"special_key": {
						Value: &model_v1.SpanData_UserPropertyValue_BoolValue{
							BoolValue: true,
						},
					},
				},
			},
			want: map[string]interface{}{
				"messaging.system":                                        "SolacePubSub+",
				"messaging.operation":                                     "receive",
				"messaging.protocol":                                      "MQTT",
				"messaging.protocol_version":                              "5.0",
				"messaging.message_id":                                    "someMessageID",
				"messaging.conversation_id":                               "someConversationID",
				"messaging.message_payload_size_bytes":                    int64(1234),
				"messaging.destination":                                   "someTopic",
				"messaging.solace.client_username":                        "someClientUsername",
				"messaging.solace.client_name":                            "someClient1234",
				"messaging.solace.replication_group_message_id":           "rmid1:00010-40910192431-40516479-90a9c4e1",
				"messaging.solace.priority":                               int64(1),
				"messaging.solace.ttl":                                    int64(86000),
				"messaging.solace.dmq_eligible":                           true,
				"messaging.solace.dropped_enqueue_events_success":         int64(42),
				"messaging.solace.dropped_enqueue_events_failed":          int64(24),
				"messaging.solace.reply_to_topic":                         "someReplyToTopic",
				"messaging.solace.delivery_mode":                          "persistent",
				"net.host.ip":                                             "1.2.3.4",
				"net.host.port":                                           int64(55555),
				"net.peer.ip":                                             "2345:425:2ca1::567:5673:23b5",
				"net.peer.port":                                           int64(12345),
				"messaging.solace.user_properties.special_key":            true,
				"messaging.solace.broker_receive_time_unix_nano":          int64(1357924680),
				"messaging.solace.dropped_application_message_properties": false,
			},
		},
		{
			name: "With Only Required Fields",
			spanData: &model_v1.SpanData{
				Protocol:                            "MQTT",
				BinaryAttachmentSize:                1000,
				XmlAttachmentSize:                   200,
				MetadataSize:                        34,
				ClientUsername:                      "someClientUsername",
				ClientName:                          "someClient1234",
				Topic:                               "someTopic",
				DeliveryMode:                        model_v1.SpanData_NON_PERSISTENT,
				DmqEligible:                         true,
				DroppedEnqueueEventsSuccess:         42,
				DroppedEnqueueEventsFailed:          24,
				HostIp:                              []byte{1, 2, 3, 4},
				HostPort:                            55555,
				PeerIp:                              []byte{35, 69, 4, 37, 44, 161, 0, 0, 0, 0, 5, 103, 86, 115, 35, 181},
				PeerPort:                            12345,
				BrokerReceiveTimeUnixNano:           1357924680,
				DroppedApplicationMessageProperties: true,
				UserProperties: map[string]*model_v1.SpanData_UserPropertyValue{
					"special_key": nil,
				},
			},
			want: map[string]interface{}{
				"messaging.system":                                        "SolacePubSub+",
				"messaging.operation":                                     "receive",
				"messaging.protocol":                                      "MQTT",
				"messaging.message_payload_size_bytes":                    int64(1234),
				"messaging.destination":                                   "someTopic",
				"messaging.solace.client_username":                        "someClientUsername",
				"messaging.solace.client_name":                            "someClient1234",
				"messaging.solace.dmq_eligible":                           true,
				"messaging.solace.delivery_mode":                          "non_persistent",
				"messaging.solace.dropped_enqueue_events_success":         int64(42),
				"messaging.solace.dropped_enqueue_events_failed":          int64(24),
				"net.host.ip":                                             "1.2.3.4",
				"net.host.port":                                           int64(55555),
				"net.peer.ip":                                             "2345:425:2ca1::567:5673:23b5",
				"net.peer.port":                                           int64(12345),
				"messaging.solace.broker_receive_time_unix_nano":          int64(1357924680),
				"messaging.solace.dropped_application_message_properties": true,
			},
		},
		{
			name: "With Some Invalid Fields",
			spanData: &model_v1.SpanData{
				Protocol:                            "MQTT",
				BinaryAttachmentSize:                1000,
				XmlAttachmentSize:                   200,
				MetadataSize:                        34,
				ClientUsername:                      "someClientUsername",
				ClientName:                          "someClient1234",
				Topic:                               "someTopic",
				DeliveryMode:                        model_v1.SpanData_DeliveryMode(1000),
				DmqEligible:                         true,
				DroppedEnqueueEventsSuccess:         42,
				DroppedEnqueueEventsFailed:          24,
				HostPort:                            55555,
				PeerPort:                            12345,
				BrokerReceiveTimeUnixNano:           1357924680,
				DroppedApplicationMessageProperties: true,
				UserProperties: map[string]*model_v1.SpanData_UserPropertyValue{
					"special_key": nil,
				},
			},
			want: map[string]interface{}{
				"messaging.system":                                        "SolacePubSub+",
				"messaging.operation":                                     "receive",
				"messaging.protocol":                                      "MQTT",
				"messaging.message_payload_size_bytes":                    int64(1234),
				"messaging.destination":                                   "someTopic",
				"messaging.solace.client_username":                        "someClientUsername",
				"messaging.solace.client_name":                            "someClient1234",
				"messaging.solace.dmq_eligible":                           true,
				"messaging.solace.delivery_mode":                          "Unknown Delivery Mode (1000)",
				"messaging.solace.dropped_enqueue_events_success":         int64(42),
				"messaging.solace.dropped_enqueue_events_failed":          int64(24),
				"net.host.port":                                           int64(55555),
				"net.peer.port":                                           int64(12345),
				"messaging.solace.broker_receive_time_unix_nano":          int64(1357924680),
				"messaging.solace.dropped_application_message_properties": true,
			},
			expectedUnmarshallingErrors: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newTestV1Unmarshaller(t)
			actual := pcommon.NewMap()
			u.mapClientSpanAttributes(tt.spanData, actual)
			assert.Equal(t, tt.want, actual.AsRaw())
			validateMetric(t, u.metrics.views.recoverableUnmarshallingErrors, tt.expectedUnmarshallingErrors)
		})
	}
}

// Validate that all event types are properly handled and appended into the span data
func TestUnmarshallerEvents(t *testing.T) {
	someErrorString := "some error"
	tests := []struct {
		name                 string
		spanData             *model_v1.SpanData
		populateExpectedSpan func(span ptrace.Span)
		unmarshallingErrors  interface{}
	}{
		{ // don't expect any events when none are present in the span data
			name:                 "No Events",
			spanData:             &model_v1.SpanData{},
			populateExpectedSpan: func(span ptrace.Span) {},
		},
		{ // when an enqueue event is present, expect it to be added to the span events
			name: "Enqueue Event Queue",
			spanData: &model_v1.SpanData{
				EnqueueEvents: []*model_v1.SpanData_EnqueueEvent{
					{
						Dest:         &model_v1.SpanData_EnqueueEvent_QueueName{QueueName: "somequeue"},
						TimeUnixNano: 123456789,
					},
				},
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "somequeue enqueue", 123456789, map[string]interface{}{
					"messaging.solace.destination_type":     "queue",
					"messaging.solace.rejects_all_enqueues": false,
				})
			},
		},
		{ // when a topic endpoint enqueue event is present, expect it to be added to the span events
			name: "Enqueue Event Topic Endpoint",
			spanData: &model_v1.SpanData{
				EnqueueEvents: []*model_v1.SpanData_EnqueueEvent{
					{
						Dest:               &model_v1.SpanData_EnqueueEvent_TopicEndpointName{TopicEndpointName: "sometopic"},
						TimeUnixNano:       123456789,
						ErrorDescription:   &someErrorString,
						RejectsAllEnqueues: true,
					},
				},
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "sometopic enqueue", 123456789, map[string]interface{}{
					"messaging.solace.destination_type":      "topic-endpoint",
					"messaging.solace.enqueue_error_message": someErrorString,
					"messaging.solace.rejects_all_enqueues":  true,
				})
			},
		},
		{ // when a both a queue and topic endpoint enqueue event is present, expect it to be added to the span events
			name: "Enqueue Event Queue and Topic Endpoint",
			spanData: &model_v1.SpanData{
				EnqueueEvents: []*model_v1.SpanData_EnqueueEvent{
					{
						Dest:         &model_v1.SpanData_EnqueueEvent_QueueName{QueueName: "somequeue"},
						TimeUnixNano: 123456789,
					},
					{
						Dest:         &model_v1.SpanData_EnqueueEvent_TopicEndpointName{TopicEndpointName: "sometopic"},
						TimeUnixNano: 2345678,
					},
				},
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "somequeue enqueue", 123456789, map[string]interface{}{
					"messaging.solace.destination_type":     "queue",
					"messaging.solace.rejects_all_enqueues": false,
				})
				populateEvent(t, span, "sometopic enqueue", 2345678, map[string]interface{}{
					"messaging.solace.destination_type":     "topic-endpoint",
					"messaging.solace.rejects_all_enqueues": false,
				})
			},
		},
		{ // when an enqueue event does not have a valid dest (ie. nil)
			name: "Enqueue Event no Dest",
			spanData: &model_v1.SpanData{
				EnqueueEvents: []*model_v1.SpanData_EnqueueEvent{
					{
						Dest:         nil,
						TimeUnixNano: 123456789,
					},
				},
			},
			populateExpectedSpan: func(span ptrace.Span) {},
			unmarshallingErrors:  1,
		},
		{ // Local Transaction
			name: "Local Transaction Event",
			spanData: &model_v1.SpanData{
				TransactionEvent: &model_v1.SpanData_TransactionEvent{
					TimeUnixNano: 123456789,
					Type:         model_v1.SpanData_TransactionEvent_COMMIT,
					Initiator:    model_v1.SpanData_TransactionEvent_CLIENT,
					TransactionId: &model_v1.SpanData_TransactionEvent_LocalId{
						LocalId: &model_v1.SpanData_TransactionEvent_LocalTransactionId{
							TransactionId: 12345,
							SessionId:     67890,
							SessionName:   "my-session-name",
						},
					},
				},
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "commit", 123456789, map[string]interface{}{
					"messaging.solace.transaction_initiator":   "client",
					"messaging.solace.transaction_id":          12345,
					"messaging.solace.transacted_session_name": "my-session-name",
					"messaging.solace.transacted_session_id":   67890,
				})
			},
		},
		{ // XA transaction
			name: "XA Transaction Event",
			spanData: &model_v1.SpanData{
				TransactionEvent: &model_v1.SpanData_TransactionEvent{
					TimeUnixNano: 123456789,
					Type:         model_v1.SpanData_TransactionEvent_END,
					Initiator:    model_v1.SpanData_TransactionEvent_ADMIN,
					TransactionId: &model_v1.SpanData_TransactionEvent_Xid_{
						Xid: &model_v1.SpanData_TransactionEvent_Xid{
							FormatId:        123,
							BranchQualifier: []byte{0, 8, 20, 254},
							GlobalId:        []byte{128, 64, 32, 16, 8, 4, 2, 1, 0},
						},
					},
				},
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "end", 123456789, map[string]interface{}{
					"messaging.solace.transaction_initiator": "administrator",
					"messaging.solace.transaction_xid":       "0000007b-000814fe-804020100804020100",
				})
			},
		},
		{ // XA Transaction with no branch qualifier or global ID and with an error
			name: "XA Transaction Event with nil fields and error",
			spanData: &model_v1.SpanData{
				TransactionEvent: &model_v1.SpanData_TransactionEvent{
					TimeUnixNano: 123456789,
					Type:         model_v1.SpanData_TransactionEvent_PREPARE,
					Initiator:    model_v1.SpanData_TransactionEvent_BROKER,
					TransactionId: &model_v1.SpanData_TransactionEvent_Xid_{
						Xid: &model_v1.SpanData_TransactionEvent_Xid{
							FormatId:        123,
							BranchQualifier: nil,
							GlobalId:        nil,
						},
					},
					ErrorDescription: &someErrorString,
				},
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "prepare", 123456789, map[string]interface{}{
					"messaging.solace.transaction_initiator":     "broker",
					"messaging.solace.transaction_xid":           "0000007b--",
					"messaging.solace.transaction_error_message": someErrorString,
				})
			},
		},
		{ // Type of transaction not handled
			name: "Unknown Transaction Type and no ID",
			spanData: &model_v1.SpanData{
				TransactionEvent: &model_v1.SpanData_TransactionEvent{
					TimeUnixNano: 123456789,
					Type:         model_v1.SpanData_TransactionEvent_Type(12345),
				},
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "Unknown Transaction Event (12345)", 123456789, map[string]interface{}{
					"messaging.solace.transaction_initiator": "client",
				})
			},
			unmarshallingErrors: 2,
		},
		{ // Type of ID not handled, type of initiator not handled
			name: "Unknown Transaction Initiator and no ID",
			spanData: &model_v1.SpanData{
				TransactionEvent: &model_v1.SpanData_TransactionEvent{
					TimeUnixNano:  123456789,
					Type:          model_v1.SpanData_TransactionEvent_ROLLBACK,
					Initiator:     model_v1.SpanData_TransactionEvent_Initiator(12345),
					TransactionId: nil,
				},
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "rollback", 123456789, map[string]interface{}{
					"messaging.solace.transaction_initiator": "Unknown Transaction Initiator (12345)",
				})
			},
			unmarshallingErrors: 2,
		},
		{ // when a both a queue and topic endpoint enqueue event is present, expect it to be added to the span events
			name: "Multiple Events",
			spanData: &model_v1.SpanData{
				EnqueueEvents: []*model_v1.SpanData_EnqueueEvent{
					{
						Dest:         &model_v1.SpanData_EnqueueEvent_QueueName{QueueName: "somequeue"},
						TimeUnixNano: 123456789,
					},
					{
						Dest:               &model_v1.SpanData_EnqueueEvent_TopicEndpointName{TopicEndpointName: "sometopic"},
						TimeUnixNano:       2345678,
						RejectsAllEnqueues: true,
					},
				},
				TransactionEvent: &model_v1.SpanData_TransactionEvent{
					TimeUnixNano: 123456789,
					Type:         model_v1.SpanData_TransactionEvent_ROLLBACK_ONLY,
					Initiator:    model_v1.SpanData_TransactionEvent_CLIENT,
					TransactionId: &model_v1.SpanData_TransactionEvent_LocalId{
						LocalId: &model_v1.SpanData_TransactionEvent_LocalTransactionId{
							TransactionId: 12345,
							SessionId:     67890,
							SessionName:   "my-session-name",
						},
					},
				},
			},
			populateExpectedSpan: func(span ptrace.Span) {
				populateEvent(t, span, "somequeue enqueue", 123456789, map[string]interface{}{
					"messaging.solace.destination_type":     "queue",
					"messaging.solace.rejects_all_enqueues": false,
				})
				populateEvent(t, span, "sometopic enqueue", 2345678, map[string]interface{}{
					"messaging.solace.destination_type":     "topic-endpoint",
					"messaging.solace.rejects_all_enqueues": true,
				})
				populateEvent(t, span, "rollback_only", 123456789, map[string]interface{}{
					"messaging.solace.transaction_initiator":   "client",
					"messaging.solace.transaction_id":          12345,
					"messaging.solace.transacted_session_name": "my-session-name",
					"messaging.solace.transacted_session_id":   67890,
				})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newTestV1Unmarshaller(t)
			expected := ptrace.NewTraces().ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
			tt.populateExpectedSpan(expected)
			actual := ptrace.NewTraces().ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
			u.mapEvents(tt.spanData, actual)
			// order is nondeterministic for attributes, so we must sort to get a valid comparison
			compareSpans(t, expected, actual)
			validateMetric(t, u.metrics.views.recoverableUnmarshallingErrors, tt.unmarshallingErrors)
		})
	}
}

func compareSpans(t *testing.T, expected, actual ptrace.Span) {
	assert.Equal(t, expected.Attributes().AsRaw(), actual.Attributes().AsRaw())
	require.Equal(t, expected.Events().Len(), actual.Events().Len())
	for i := 0; i < expected.Events().Len(); i++ {
		lessFunc := func(a, b ptrace.SpanEvent) bool {
			return a.Name() < b.Name() // choose any comparison here
		}
		expectedEvent := expected.Events().Sort(lessFunc).At(i)
		actualEvent := actual.Events().Sort(lessFunc).At(i)
		assert.Equal(t, expectedEvent.Name(), actualEvent.Name())
		assert.Equal(t, expectedEvent.Timestamp(), actualEvent.Timestamp())
		assert.Equal(t, expectedEvent.Attributes().AsRaw(), actualEvent.Attributes().AsRaw())
	}
}

func populateEvent(t *testing.T, span ptrace.Span, name string, timestamp uint64, attributes map[string]interface{}) {
	spanEvent := span.Events().AppendEmpty()
	spanEvent.SetName(name)
	spanEvent.SetTimestamp(pcommon.Timestamp(timestamp))
	populateAttributes(t, spanEvent.Attributes(), attributes)
}

func populateAttributes(t *testing.T, attrMap pcommon.Map, attributes map[string]interface{}) {
	for key, val := range attributes {
		switch casted := val.(type) {
		case string:
			attrMap.PutStr(key, casted)
		case int64:
			attrMap.PutInt(key, casted)
		case int:
			attrMap.PutInt(key, int64(casted))
		case bool:
			attrMap.PutBool(key, casted)
		default:
			require.Fail(t, "Test setup issue: unknown type, could not insert data")
		}
	}
}

func TestUnmarshallerRGMID(t *testing.T) {
	tests := []struct {
		name     string
		in       []byte
		expected string
		numErr   interface{}
	}{
		{
			name:     "Valid RGMID",
			in:       []byte{0x01, 0x00, 0x01, 0x04, 0x09, 0x10, 0x19, 0x24, 0x31, 0x40, 0x51, 0x64, 0x79, 0x90, 0xa9, 0xc4, 0xe1},
			expected: "rmid1:00010-40910192431-40516479-90a9c4e1",
		},
		{
			name:     "Bad RGMID Version",
			in:       []byte{0x02, 0x00, 0x01, 0x04, 0x09, 0x10, 0x19, 0x24, 0x31, 0x40, 0x51, 0x64, 0x79, 0x90, 0xa9, 0xc4, 0xe1},
			expected: "0200010409101924314051647990a9c4e1", // expect default behavior of hex dump
			numErr:   1,
		},
		{
			name:     "Bad RGMID length",
			in:       []byte{0x00, 0x01, 0x04, 0x09, 0x10, 0x19, 0x24, 0x31, 0x40, 0x51, 0x64, 0x79, 0x90, 0xa9, 0xc4, 0xe1},
			expected: "00010409101924314051647990a9c4e1", // expect default behavior of hex dump
			numErr:   1,
		},
		{
			name:     "Nil RGMID",
			in:       nil,
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newTestV1Unmarshaller(t)
			actual := u.rgmidToString(tt.in)
			assert.Equal(t, tt.expected, actual)
			validateMetric(t, u.metrics.views.recoverableUnmarshallingErrors, tt.numErr)
		})
	}
}

func TestUnmarshallerInsertUserProperty(t *testing.T) {
	emojiVal := 0xf09f92a9
	testCases := []struct {
		data         interface{}
		expectedType pcommon.ValueType
		validate     func(val pcommon.Value)
	}{
		{
			&model_v1.SpanData_UserPropertyValue_NullValue{},
			pcommon.ValueTypeEmpty,
			nil,
		},
		{
			&model_v1.SpanData_UserPropertyValue_BoolValue{BoolValue: true},
			pcommon.ValueTypeBool,
			func(val pcommon.Value) {
				assert.Equal(t, true, val.Bool())
			},
		},
		{
			&model_v1.SpanData_UserPropertyValue_DoubleValue{DoubleValue: 12.34},
			pcommon.ValueTypeDouble,
			func(val pcommon.Value) {
				assert.Equal(t, float64(12.34), val.Double())
			},
		},
		{
			&model_v1.SpanData_UserPropertyValue_ByteArrayValue{ByteArrayValue: []byte{1, 2, 3, 4}},
			pcommon.ValueTypeBytes,
			func(val pcommon.Value) {
				assert.Equal(t, []byte{1, 2, 3, 4}, val.Bytes().AsRaw())
			},
		},
		{
			&model_v1.SpanData_UserPropertyValue_FloatValue{FloatValue: 12.34},
			pcommon.ValueTypeDouble,
			func(val pcommon.Value) {
				assert.Equal(t, float64(float32(12.34)), val.Double())
			},
		},
		{
			&model_v1.SpanData_UserPropertyValue_Int8Value{Int8Value: 8},
			pcommon.ValueTypeInt,
			func(val pcommon.Value) {
				assert.Equal(t, int64(8), val.Int())
			},
		},
		{
			&model_v1.SpanData_UserPropertyValue_Int16Value{Int16Value: 16},
			pcommon.ValueTypeInt,
			func(val pcommon.Value) {
				assert.Equal(t, int64(16), val.Int())
			},
		},
		{
			&model_v1.SpanData_UserPropertyValue_Int32Value{Int32Value: 32},
			pcommon.ValueTypeInt,
			func(val pcommon.Value) {
				assert.Equal(t, int64(32), val.Int())
			},
		},
		{
			&model_v1.SpanData_UserPropertyValue_Int64Value{Int64Value: 64},
			pcommon.ValueTypeInt,
			func(val pcommon.Value) {
				assert.Equal(t, int64(64), val.Int())
			},
		},
		{
			&model_v1.SpanData_UserPropertyValue_Uint8Value{Uint8Value: 8},
			pcommon.ValueTypeInt,
			func(val pcommon.Value) {
				assert.Equal(t, int64(8), val.Int())
			},
		},
		{
			&model_v1.SpanData_UserPropertyValue_Uint16Value{Uint16Value: 16},
			pcommon.ValueTypeInt,
			func(val pcommon.Value) {
				assert.Equal(t, int64(16), val.Int())
			},
		},
		{
			&model_v1.SpanData_UserPropertyValue_Uint32Value{Uint32Value: 32},
			pcommon.ValueTypeInt,
			func(val pcommon.Value) {
				assert.Equal(t, int64(32), val.Int())
			},
		},
		{
			&model_v1.SpanData_UserPropertyValue_Uint64Value{Uint64Value: 64},
			pcommon.ValueTypeInt,
			func(val pcommon.Value) {
				assert.Equal(t, int64(64), val.Int())
			},
		},
		{
			&model_v1.SpanData_UserPropertyValue_StringValue{StringValue: "hello world"},
			pcommon.ValueTypeStr,
			func(val pcommon.Value) {
				assert.Equal(t, "hello world", val.Str())
			},
		},
		{
			&model_v1.SpanData_UserPropertyValue_DestinationValue{DestinationValue: "some_dest"},
			pcommon.ValueTypeStr,
			func(val pcommon.Value) {
				assert.Equal(t, "some_dest", val.Str())
			},
		},
		{
			&model_v1.SpanData_UserPropertyValue_CharacterValue{CharacterValue: 0x61},
			pcommon.ValueTypeStr,
			func(val pcommon.Value) {
				assert.Equal(t, "a", val.Str())
			},
		},
		{
			&model_v1.SpanData_UserPropertyValue_CharacterValue{CharacterValue: 0xe68080},
			pcommon.ValueTypeStr,
			func(val pcommon.Value) {
				assert.Equal(t, string(rune(0xe68080)), val.Str())
			},
		},
		{
			&model_v1.SpanData_UserPropertyValue_CharacterValue{CharacterValue: 0xf09f92a9},
			pcommon.ValueTypeStr,
			func(val pcommon.Value) {
				assert.Equal(t, string(rune(emojiVal)), val.Str())
			},
		},
	}

	unmarshaller := &solaceMessageUnmarshallerV1{
		logger: zap.NewNop(),
	}
	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("%T", testCase.data), func(t *testing.T) {
			const key = "some-property"
			attributeMap := pcommon.NewMap()
			unmarshaller.insertUserProperty(attributeMap, key, testCase.data)
			actual, ok := attributeMap.Get("messaging.solace.user_properties." + key)
			require.True(t, ok)
			assert.Equal(t, testCase.expectedType, actual.Type())
			if testCase.validate != nil {
				testCase.validate(actual)
			}
		})
	}
}

func TestSolaceMessageUnmarshallerV1InsertUserPropertyUnsupportedType(t *testing.T) {
	u := newTestV1Unmarshaller(t)
	const key = "some-property"
	attributeMap := pcommon.NewMap()
	u.insertUserProperty(attributeMap, key, "invalid data type")
	_, ok := attributeMap.Get("messaging.solace.user_properties." + key)
	assert.False(t, ok)
	validateMetric(t, u.metrics.views.recoverableUnmarshallingErrors, 1)
}

func newTestV1Unmarshaller(t *testing.T) *solaceMessageUnmarshallerV1 {
	m := newTestMetrics(t)
	return &solaceMessageUnmarshallerV1{zap.NewNop(), m}
}
