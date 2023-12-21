/*
 * Copyright 2023 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package clientpool

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testCloseable struct {
	lock   sync.Mutex
	data   int
	closed bool
}

func (tc *testCloseable) Close() error {
	tc.lock.Lock()
	defer tc.lock.Unlock()
	if tc.closed == true {
		return fmt.Errorf("can't close an already closed value")
	}
	tc.closed = true
	return nil
}

func TestAddThreeClientsAndAcquire(t *testing.T) {
	pool := NewLRUCache[string, *testCloseable]()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)

	value1, returnValue1, exists, err := pool.AddAndAcquire(ctx, "1", func() (*testCloseable, error) {
		return &testCloseable{data: 1}, nil
	}, 8)

	assert.NoError(t, err)
	assert.Equal(t, 1, value1.data)
	assert.False(t, exists)

	value2, returnValue2, exists, err := pool.AddAndAcquire(ctx, "2", func() (*testCloseable, error) {
		return &testCloseable{data: 2}, nil
	}, 8)

	assert.NoError(t, err)
	assert.Equal(t, 2, value2.data)
	assert.False(t, exists)

	value3, returnValue3, exists, err := pool.AddAndAcquire(ctx, "3", func() (*testCloseable, error) {
		return &testCloseable{data: 3}, nil
	}, 8)

	assert.NoError(t, err)
	assert.Equal(t, 3, value3.data)
	assert.False(t, exists)

	returnValue1()
	returnValue2()
	returnValue3()

	cancel()
}

func TestAddSameKeyTwice(t *testing.T) {
	pool := NewLRUCache[string, *testCloseable]()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)

	value1, returnValue1, exists, err := pool.AddAndAcquire(ctx, "1", func() (*testCloseable, error) {
		return &testCloseable{data: 1}, nil
	}, 8)

	assert.NoError(t, err)
	assert.Equal(t, 1, value1.data)
	assert.False(t, exists)

	value2, returnValue2, exists, err := pool.AddAndAcquire(ctx, "1", func() (*testCloseable, error) {
		return &testCloseable{data: 1}, nil
	}, 8)

	assert.NoError(t, err)
	assert.Equal(t, 1, value2.data)
	assert.True(t, exists)

	returnValue1()
	returnValue2()

	cancel()
}

func TestAddAndGetKeyWithoutReturningClient(t *testing.T) {
	pool := NewLRUCache[string, *testCloseable]()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)

	value1, returnValue1, exists, err := pool.AddAndAcquire(ctx, "1", func() (*testCloseable, error) {
		return &testCloseable{data: 1}, nil
	}, 8)

	assert.NoError(t, err)
	assert.Equal(t, 1, value1.data)
	assert.False(t, exists)

	value2, returnValue2, exists, err := pool.Get(ctx, "1")

	assert.NoError(t, err)
	assert.Equal(t, 1, value2.data)
	assert.True(t, exists)

	returnValue1()
	returnValue2()

	cancel()
}

func TestNoMoreGetsUntilReturn(t *testing.T) {
	pool := NewLRUCache[string, *testCloseable]()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)

	value1, returnValue1, exists, err := pool.AddAndAcquire(ctx, "1", func() (*testCloseable, error) {
		return &testCloseable{data: 1}, nil
	}, 2)

	assert.NoError(t, err)
	assert.Equal(t, 1, value1.data)
	assert.False(t, exists)

	value2, returnValue2, exists, err := pool.Get(ctx, "1")

	assert.NoError(t, err)
	assert.Equal(t, 1, value2.data)
	assert.True(t, exists)

	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Millisecond*250)

	_, returnValue3, exists, err := pool.Get(ctx2, "1")

	assert.Error(t, err)
	assert.True(t, exists)

	returnValue3()
	returnValue1()

	value3, returnValue3, exists, err := pool.Get(ctx, "1")

	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, 1, value3.data)

	assert.NoError(t, err)

	returnValue2()
	returnValue3()

	cancel2()
	cancel()
}

func TestGetBeforeAdd(t *testing.T) {
	pool := NewLRUCache[string, *testCloseable]()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)

	_, _, exists, err := pool.Get(ctx, "1")

	assert.NoError(t, err)
	assert.False(t, exists)

	cancel()
}

func TestUpdate(t *testing.T) {
	pool := NewLRUCache[string, *testCloseable]()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)

	value1, returnValue1, exists, err := pool.AddAndAcquire(ctx, "1", func() (*testCloseable, error) {
		return &testCloseable{data: 1}, nil
	}, 2)

	assert.NoError(t, err)
	assert.Equal(t, 1, value1.data)
	assert.False(t, exists)

	value2, returnValue2, exists, err := pool.Get(ctx, "1")

	assert.NoError(t, err)
	assert.Equal(t, 1, value2.data)
	assert.True(t, exists)

	returnValue1()
	returnValue2()

	exists, err = pool.UpdateIfExists("1", func() (*testCloseable, error) {
		return &testCloseable{
			data: 2,
		}, nil
	}, 2)

	assert.NoError(t, err)
	assert.True(t, exists)

	value3, returnValue3, exists, err := pool.Get(ctx, "1")

	assert.NoError(t, err)
	assert.Equal(t, 2, value3.data)
	assert.True(t, exists)

	returnValue3()

	cancel()
}
