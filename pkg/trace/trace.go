// Copyright 2016 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package trace

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/vmware/vic/pkg/log"
)

var Logger = &logrus.Logger{
	Out: os.Stderr,
	// We're using our own text formatter to skip the \n and \t escaping logrus
	// was doing on non TTY Out (we redirect to a file) descriptors.
	Formatter: &log.TextFormatter{
		Timestamp:       true,
		TimestampFormat: "2006-01-02T15:04:05.000000000Z07:00",
	},
	Hooks: make(logrus.LevelHooks),
	Level: logrus.InfoLevel,
}

// trace object used to grab run-time state
type Message struct {
	op       *Operation
	msg      string
	funcName string
	lineNo   int

	startTime time.Time
}

const precision = 10 * time.Microsecond

func (t *Message) delta() time.Duration {
	return time.Now().Truncate(precision).Sub(t.startTime.Truncate(precision))
}

func (t *Message) beginHdr() string {
	return fmt.Sprintf("[BEGIN] [%s:%d]", t.funcName, t.lineNo)
}

func (t *Message) endHdr() string {
	return fmt.Sprintf("[ END ] [%s:%d]", t.funcName, t.lineNo)
}

// begin a trace from this stack frame less the skip.
func newTrace(skip int, format string, args ...interface{}) *Message {
	pc, _, line, ok := runtime.Caller(skip)
	if !ok {
		return nil
	}

	name := runtime.FuncForPC(pc).Name()

	t := &Message{
		msg:       format,
		funcName:  name,
		lineNo:    line,
		startTime: time.Now(),
	}

	if format != "" {
		t.msg = fmt.Sprintf(format, args...)
	}

	return t
}

// Begin starts the trace.  Msg is the msg to log.
func Begin(msg string) *Message {
	t := newTrace(2, msg)

	if msg == "" {
		Logger.Debugf(t.beginHdr())
	} else {
		Logger.Debugf("%s %s", t.beginHdr(), t.msg)
	}

	return t
}

func BeginOp(op *Operation, format string, args ...interface{}) *Message {
	t := newTrace(2, format, args...)

	if format == "" {
		op.Debugf(t.beginHdr())
	} else {
		op.Debugf("%s %s", t.beginHdr(), t.msg)
	}

	t.op = op
	return t
}

// End ends the trace.
func End(t *Message) {
	if t.op != nil {
		t.op.Debugf("%s %s", t.endHdr(), t.msg)
		return
	}

	Logger.Debugf("%s [%s] %s", t.endHdr(), t.delta(), t.msg)
}
