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

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	eventhub "github.com/Azure/azure-event-hubs-go/v3"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type rawConverter struct{}

func newRawConverter(_ component.ReceiverCreateSettings) *rawConverter {
	return &rawConverter{}
}

func (*rawConverter) ToLogs(event *eventhub.Event) (plog.Logs, error) {
	l := plog.NewLogs()
	lr := l.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	slice := lr.Body().SetEmptyBytes()
	slice.Append(event.Data...)
	if event.SystemProperties.EnqueuedTime != nil {
		lr.SetTimestamp(pcommon.NewTimestampFromTime(*event.SystemProperties.EnqueuedTime))
	}
	if err := lr.Attributes().FromRaw(event.Properties); err != nil {
		return l, err
	}
	return l, nil
}
