// Copyright 2013 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package scrape

import (
	"context"
	"math/rand"

	"github.com/adolli/prometheus/pkg/exemplar"
	"github.com/adolli/prometheus/pkg/labels"
	"github.com/adolli/prometheus/storage"
)

type nopAppendable struct{}

func (a nopAppendable) Appender(_ context.Context) storage.Appender {
	return nopAppender{}
}

type nopAppender struct{}

func (a nopAppender) Append(uint64, labels.Labels, int64, float64) (uint64, error) { return 0, nil }
func (a nopAppender) AppendExemplar(uint64, labels.Labels, exemplar.Exemplar) (uint64, error) {
	return 0, nil
}
func (a nopAppender) Commit() error   { return nil }
func (a nopAppender) Rollback() error { return nil }

type sample struct {
	metric labels.Labels
	t      int64
	v      float64
}

// collectResultAppender records all samples that were added through the appender.
// It can be used as its zero value or be backed by another appender it writes samples through.
type collectResultAppender struct {
	next             storage.Appender
	result           []sample
	pendingResult    []sample
	rolledbackResult []sample
	pendingExemplars []exemplar.Exemplar
	resultExemplars  []exemplar.Exemplar
}

func (a *collectResultAppender) Append(ref uint64, lset labels.Labels, t int64, v float64) (uint64, error) {
	a.pendingResult = append(a.pendingResult, sample{
		metric: lset,
		t:      t,
		v:      v,
	})

	if ref == 0 {
		ref = rand.Uint64()
	}
	if a.next == nil {
		return ref, nil
	}

	ref, err := a.next.Append(ref, lset, t, v)
	if err != nil {
		return 0, err
	}
	return ref, err
}

func (a *collectResultAppender) AppendExemplar(ref uint64, l labels.Labels, e exemplar.Exemplar) (uint64, error) {
	a.pendingExemplars = append(a.pendingExemplars, e)
	if a.next == nil {
		return 0, nil
	}

	return a.next.AppendExemplar(ref, l, e)
}

func (a *collectResultAppender) Commit() error {
	a.result = append(a.result, a.pendingResult...)
	a.resultExemplars = append(a.resultExemplars, a.pendingExemplars...)
	a.pendingResult = nil
	a.pendingExemplars = nil
	if a.next == nil {
		return nil
	}
	return a.next.Commit()
}

func (a *collectResultAppender) Rollback() error {
	a.rolledbackResult = a.pendingResult
	a.pendingResult = nil
	if a.next == nil {
		return nil
	}
	return a.next.Rollback()
}
