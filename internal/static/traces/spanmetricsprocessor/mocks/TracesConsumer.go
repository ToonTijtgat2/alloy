// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// TracesConsumer is an autogenerated mock type for the TracesConsumer type
type TracesConsumer struct {
	mock.Mock
}

func (_m *TracesConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeTraces provides a mock function with given fields: ctx, td
func (_m *TracesConsumer) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	ret := _m.Called(ctx, td)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, ptrace.Traces) error); ok {
		r0 = rf(ctx, td)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}