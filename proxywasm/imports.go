/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package proxywasm

import (
	"net/url"
	"sync"
	"sync/atomic"

	"mosn.io/api"
	"mosn.io/pkg/buffer"
	"mosn.io/proxy-wasm-go-host/types"
)

func RegisterImports(instance types.WasmInstance) {
	_ = instance.RegisterFunc("env", "proxy_log", proxyLog)

	_ = instance.RegisterFunc("env", "proxy_set_effective_context", proxySetEffectiveContext)

	_ = instance.RegisterFunc("env", "proxy_get_property", proxyGetProperty)
	_ = instance.RegisterFunc("env", "proxy_set_property", proxySetProperty)

	_ = instance.RegisterFunc("env", "proxy_get_buffer_bytes", proxyGetBufferBytes)
	_ = instance.RegisterFunc("env", "proxy_set_buffer_bytes", proxySetBufferBytes)

	_ = instance.RegisterFunc("env", "proxy_get_header_map_pairs", proxyGetHeaderMapPairs)
	_ = instance.RegisterFunc("env", "proxy_set_header_map_pairs", proxySetHeaderMapPairs)

	_ = instance.RegisterFunc("env", "proxy_get_header_map_value", proxyGetHeaderMapValue)
	_ = instance.RegisterFunc("env", "proxy_replace_header_map_value", proxyReplaceHeaderMapValue)
	_ = instance.RegisterFunc("env", "proxy_add_header_map_value", proxyAddHeaderMapValue)
	_ = instance.RegisterFunc("env", "proxy_remove_header_map_value", proxyRemoveHeaderMapValue)

	_ = instance.RegisterFunc("env", "proxy_set_tick_period_milliseconds", proxySetTickPeriodMilliseconds)
	_ = instance.RegisterFunc("env", "proxy_get_current_time_nanoseconds", proxyGetCurrentTimeNanoseconds)

	_ = instance.RegisterFunc("env", "proxy_grpc_call", proxyGrpcCall)
	_ = instance.RegisterFunc("env", "proxy_grpc_stream", proxyGrpcStream)
	_ = instance.RegisterFunc("env", "proxy_grpc_cancel", proxyGrpcCancel)
	_ = instance.RegisterFunc("env", "proxy_grpc_close", proxyGrpcClose)
	_ = instance.RegisterFunc("env", "proxy_grpc_send", proxyGrpcSend)

	_ = instance.RegisterFunc("env", "proxy_http_call", proxyHttpCall)

	_ = instance.RegisterFunc("env", "proxy_define_metric", proxyDefineMetric)
	_ = instance.RegisterFunc("env", "proxy_increment_metric", proxyIncrementMetric)
	_ = instance.RegisterFunc("env", "proxy_record_metric", proxyRecordMetric)
	_ = instance.RegisterFunc("env", "proxy_get_metric", proxyGetMetric)

	_ = instance.RegisterFunc("env", "proxy_register_shared_queue", proxyRegisterSharedQueue)
	_ = instance.RegisterFunc("env", "proxy_resolve_shared_queue", proxyResolveSharedQueue)
	_ = instance.RegisterFunc("env", "proxy_dequeue_shared_queue", proxyDequeueSharedQueue)
	_ = instance.RegisterFunc("env", "proxy_enqueue_shared_queue", proxyEnqueueSharedQueue)

	_ = instance.RegisterFunc("env", "proxy_get_shared_data", proxyGetSharedData)
	_ = instance.RegisterFunc("env", "proxy_set_shared_data", proxySetSharedData)
}

func getInstanceCallback(instance types.WasmInstance) ImportsHandler {
	v := instance.GetData()
	if v == nil {
		return &DefaultImportsHandler{}
	}

	cb, ok := v.(*ABIContext)
	if !ok {
		return &DefaultImportsHandler{}
	}

	if cb.Imports == nil {
		return &DefaultImportsHandler{}
	}

	return cb.Imports
}

func GetBuffer(instance types.WasmInstance, bufferType BufferType) buffer.IoBuffer {
	switch bufferType {
	case BufferTypeHttpRequestBody:
		callback := getInstanceCallback(instance)
		return callback.GetHttpRequestBody()
	case BufferTypeHttpResponseBody:
		callback := getInstanceCallback(instance)
		return callback.GetHttpResponseBody()
	case BufferTypePluginConfiguration:
		callback := getInstanceCallback(instance)
		return callback.GetPluginConfig()
	case BufferTypeVmConfiguration:
		callback := getInstanceCallback(instance)
		return callback.GetVmConfig()
	case BufferTypeHttpCallResponseBody:
		return nil
	}

	return nil
}

func GetMap(instance types.WasmInstance, mapType MapType) api.HeaderMap {
	switch mapType {
	case MapTypeHttpRequestHeaders:
		callback := getInstanceCallback(instance)
		return callback.GetHttpRequestHeader()
	case MapTypeHttpRequestTrailers:
		callback := getInstanceCallback(instance)
		return callback.GetHttpRequestTrailer()
	case MapTypeHttpResponseHeaders:
		callback := getInstanceCallback(instance)
		return callback.GetHttpResponseHeader()
	case MapTypeHttpResponseTrailers:
		callback := getInstanceCallback(instance)
		return callback.GetHttpResponseTrailer()
	case MapTypeHttpCallResponseHeaders:
		return nil
	case MapTypeHttpCallResponseTrailers:
		return nil
	}

	return nil
}

func proxyLog(instance types.WasmInstance, level int32, logDataPtr int32, logDataSize int32) int32 {
	callback := getInstanceCallback(instance)

	logContent, err := instance.GetMemory(uint64(logDataPtr), uint64(logDataSize))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}

	callback.Log(LogLevel(level), string(logContent))

	return WasmResultOk.Int32()
}

func proxyGetBufferBytes(instance types.WasmInstance, bufferType int32, start int32, length int32, returnBufferData int32, returnBufferSize int32) int32 {
	if BufferType(bufferType) > BufferTypeMax {
		return WasmResultBadArgument.Int32()
	}

	buf := GetBuffer(instance, BufferType(bufferType))
	if buf == nil {
		return WasmResultNotFound.Int32()
	}

	if start > start+length {
		return WasmResultBadArgument.Int32()
	}

	if start+length > int32(buf.Len()) {
		length = int32(buf.Len()) - start
	}

	addr, err := instance.Malloc(int32(length))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}

	err = instance.PutMemory(addr, uint64(length), buf.Bytes())
	if err != nil {
		return WasmResultInternalFailure.Int32()
	}

	err = instance.PutUint32(uint64(returnBufferData), uint32(addr))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}

	err = instance.PutUint32(uint64(returnBufferSize), uint32(length))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}

	return WasmResultOk.Int32()
}

func proxySetBufferBytes(instance types.WasmInstance, bufferType int32, start int32, length int32, dataPtr int32, dataSize int32) int32 {
	if BufferType(bufferType) > BufferTypeMax {
		return WasmResultBadArgument.Int32()
	}

	buf := GetBuffer(instance, BufferType(bufferType))
	if buf == nil {
		return WasmResultNotFound.Int32()
	}

	content, err := instance.GetMemory(uint64(dataPtr), uint64(dataSize))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}

	if start == 0 {
		if length == 0 || int(length) >= buf.Len() {
			buf.Drain(buf.Len())
			_, err = buf.Write(content)
		} else {
			return WasmResultBadArgument.Int32()
		}
	} else if int(start) >= buf.Len() {
		_, err = buf.Write(content)
	} else {
		return WasmResultBadArgument.Int32()
	}

	if err != nil {
		return WasmResultInternalFailure.Int32()
	}

	return WasmResultOk.Int32()
}

func proxyGetHeaderMapPairs(instance types.WasmInstance, mapType int32, returnDataPtr int32, returnDataSize int32) int32 {
	if MapType(mapType) > MapTypeMax {
		return WasmResultBadArgument.Int32()
	}

	header := GetMap(instance, MapType(mapType))
	if header == nil {
		return WasmResultNotFound.Int32()
	}

	cloneMap := make(map[string]string)
	totalBytesLen := 4
	header.Range(func(key, value string) bool {
		cloneMap[key] = value
		totalBytesLen += 4 + 4                         // keyLen + valueLen
		totalBytesLen += len(key) + 1 + len(value) + 1 // key + \0 + value + \0
		return true
	})

	addr, err := instance.Malloc(int32(totalBytesLen))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}

	err = instance.PutUint32(addr, uint32(len(cloneMap)))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}

	lenPtr := addr + 4
	dataPtr := lenPtr + uint64(8*len(cloneMap))

	for k, v := range cloneMap {
		_ = instance.PutUint32(lenPtr, uint32(len(k)))
		lenPtr += 4
		_ = instance.PutUint32(lenPtr, uint32(len(v)))
		lenPtr += 4

		_ = instance.PutMemory(dataPtr, uint64(len(k)), []byte(k))
		dataPtr += uint64(len(k))
		_ = instance.PutByte(dataPtr, 0)
		dataPtr++

		_ = instance.PutMemory(dataPtr, uint64(len(v)), []byte(v))
		dataPtr += uint64(len(v))
		_ = instance.PutByte(dataPtr, 0)
		dataPtr++
	}

	err = instance.PutUint32(uint64(returnDataPtr), uint32(addr))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}

	err = instance.PutUint32(uint64(returnDataSize), uint32(totalBytesLen))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}

	return WasmResultOk.Int32()
}

func proxySetHeaderMapPairs(instance types.WasmInstance, mapType int32, ptr int32, size int32) int32 {
	if MapType(mapType) > MapTypeMax {
		return WasmResultBadArgument.Int32()
	}

	headerMap := GetMap(instance, MapType(mapType))
	if headerMap == nil {
		return WasmResultNotFound.Int32()
	}

	newMapContent, err := instance.GetMemory(uint64(ptr), uint64(size))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}

	newMap := DecodeMap(newMapContent)

	for k, v := range newMap {
		headerMap.Set(k, v)
	}

	return WasmResultOk.Int32()
}

func proxyGetHeaderMapValue(instance types.WasmInstance, mapType int32, keyDataPtr int32, keySize int32, valueDataPtr int32, valueSize int32) int32 {
	if MapType(mapType) > MapTypeMax {
		return WasmResultBadArgument.Int32()
	}

	headerMap := GetMap(instance, MapType(mapType))
	if headerMap == nil {
		return WasmResultNotFound.Int32()
	}

	key, err := instance.GetMemory(uint64(keyDataPtr), uint64(keySize))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}
	if len(key) == 0 {
		return WasmResultBadArgument.Int32()
	}

	value, ok := headerMap.Get(string(key))
	if !ok {
		return WasmResultNotFound.Int32()
	}

	addr, err := instance.Malloc(int32(len(value)))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}

	err = instance.PutMemory(addr, uint64(len(value)), []byte(value))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}

	err = instance.PutUint32(uint64(valueDataPtr), uint32(addr))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}

	err = instance.PutUint32(uint64(valueSize), uint32(len(value)))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}

	return WasmResultOk.Int32()
}

func proxyReplaceHeaderMapValue(instance types.WasmInstance, mapType int32, keyDataPtr int32, keySize int32, valueDataPtr int32, valueSize int32) int32 {
	if MapType(mapType) > MapTypeMax {
		return WasmResultBadArgument.Int32()
	}

	headerMap := GetMap(instance, MapType(mapType))
	if headerMap == nil {
		return WasmResultNotFound.Int32()
	}

	key, err := instance.GetMemory(uint64(keyDataPtr), uint64(keySize))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}
	if len(key) == 0 {
		return WasmResultBadArgument.Int32()
	}

	value, err := instance.GetMemory(uint64(valueDataPtr), uint64(valueSize))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}
	if len(value) == 0 {
		return WasmResultBadArgument.Int32()
	}

	headerMap.Set(string(key), string(value))

	return WasmResultOk.Int32()
}

func proxyAddHeaderMapValue(instance types.WasmInstance, mapType int32, keyDataPtr int32, keySize int32, valueDataPtr int32, valueSize int32) int32 {
	if MapType(mapType) > MapTypeMax {
		return WasmResultBadArgument.Int32()
	}

	headerMap := GetMap(instance, MapType(mapType))
	if headerMap == nil {
		return WasmResultNotFound.Int32()
	}

	key, err := instance.GetMemory(uint64(keyDataPtr), uint64(keySize))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}
	if len(key) == 0 {
		return WasmResultBadArgument.Int32()
	}

	value, err := instance.GetMemory(uint64(valueDataPtr), uint64(valueSize))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}
	if len(value) == 0 {
		return WasmResultBadArgument.Int32()
	}

	headerMap.Set(string(key), string(value))

	return WasmResultOk.Int32()
}

func proxyRemoveHeaderMapValue(instance types.WasmInstance, mapType int32, keyDataPtr int32, keySize int32) int32 {
	if MapType(mapType) > MapTypeMax {
		return WasmResultBadArgument.Int32()
	}

	headerMap := GetMap(instance, MapType(mapType))
	if headerMap == nil {
		return WasmResultNotFound.Int32()
	}

	key, err := instance.GetMemory(uint64(keyDataPtr), uint64(keySize))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}
	if len(key) == 0 {
		return WasmResultBadArgument.Int32()
	}

	headerMap.Del(string(key))

	return WasmResultOk.Int32()
}

func proxyGetProperty(instance types.WasmInstance, keyPtr int32, keySize int32, returnValueData int32, returnValueSize int32) int32 {
	return WasmResultOk.Int32()
}

func proxySetProperty(instance types.WasmInstance, keyPtr int32, keySize int32, valuePtr int32, valueSize int32) int32 {
	return WasmResultUnimplemented.Int32()
}

func proxySetEffectiveContext(instance types.WasmInstance, contextID int32) int32 {
	return WasmResultUnimplemented.Int32()
}

func proxySetTickPeriodMilliseconds(instance types.WasmInstance, tickPeriodMilliseconds int32) int32 {
	return WasmResultUnimplemented.Int32()
}

func proxyGetCurrentTimeNanoseconds(instance types.WasmInstance, resultUint64Ptr int32) int32 {
	return WasmResultUnimplemented.Int32()
}

func proxyGrpcCall(instance types.WasmInstance, servicePtr int32, serviceSize int32, serviceNamePtr int32, serviceNameSize int32,
	methodNamePtr int32, methodNameSize int32,
	initialMetadataPtr int32, initialMetadataSize int32,
	requestPtr int32, requestSize int32,
	timeoutMilliseconds int32, tokenPtr int32) int32 {
	return WasmResultUnimplemented.Int32()
}

func proxyGrpcStream(instance types.WasmInstance, servicePtr int32, serviceSize int32, serviceNamePtr int32, serviceNameSize int32,
	methodNamePtr int32, methodNameSize int32,
	initialMetadataPtr int32, initialMetadataSize int32,
	tokenPtr int32) int32 {
	return WasmResultUnimplemented.Int32()
}

func proxyGrpcCancel(instance types.WasmInstance, token int32) int32 {
	return WasmResultUnimplemented.Int32()
}

func proxyGrpcClose(instance types.WasmInstance, token int32) int32 {
	return WasmResultUnimplemented.Int32()
}

func proxyGrpcSend(instance types.WasmInstance, token int32, messagePtr int32, messageSize int32, endStream int32) int32 {
	return WasmResultUnimplemented.Int32()
}

func parseHttpCalloutReq(instance types.WasmInstance, headerPairsPtr int32, headerPairsSize int32,
	bodyPtr int32, bodySize int32, trailerPairsPtr int32, trailerPairsSize int32) (api.HeaderMap, buffer.IoBuffer, api.HeaderMap, error) {
	var header api.HeaderMap
	if headerPairsSize > 0 {
		headerStr, err := instance.GetMemory(uint64(headerPairsPtr), uint64(headerPairsSize))
		if err != nil {
			return nil, nil, nil, err
		}

		headerMap := DecodeMap(headerStr)
		for k, v := range headerMap {
			header.Add(k, v)
		}
	}

	var body buffer.IoBuffer
	if bodySize > 0 {
		bodyStr, err := instance.GetMemory(uint64(bodyPtr), uint64(bodySize))
		if err != nil {
			return nil, nil, nil, err
		}

		body = buffer.NewIoBufferBytes(bodyStr)
	}

	var trailer api.HeaderMap
	if trailerPairsSize > 0 {
		trailerStr, err := instance.GetMemory(uint64(trailerPairsPtr), uint64(trailerPairsSize))
		if err != nil {
			return nil, nil, nil, err
		}

		trailerMap := DecodeMap(trailerStr)
		if trailerMap != nil {
			for k, v := range trailerMap {
				trailer.Add(k, v)
			}
		}
	}

	return header, body, trailer, nil
}

var httpCalloutID int32

func proxyHttpCall(instance types.WasmInstance, uriPtr int32, uriSize int32,
	headerPairsPtr int32, headerPairsSize int32,
	bodyPtr int32, bodySize int32,
	trailerPairsPtr int32, trailerPairsSize int32,
	timeoutMilliseconds int32, calloutIDPtr int32) int32 {
	urlStr, err := instance.GetMemory(uint64(uriPtr), uint64(uriSize))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}

	u, err := url.Parse(string(urlStr))
	if err != nil {
		return WasmResultBadArgument.Int32()
	}

	header, body, trailer, err := parseHttpCalloutReq(instance, headerPairsPtr, headerPairsSize, bodyPtr, bodySize, trailerPairsPtr, trailerPairsSize)
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}
	_, _, _, _ = header, body, trailer, u

	calloutID := atomic.AddInt32(&httpCalloutID, 1)

	err = instance.PutUint32(uint64(calloutIDPtr), uint32(calloutID))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}

	return WasmResultOk.Int32()
}

func proxyDefineMetric(instance types.WasmInstance, metricType int32, namePtr int32, nameSize int32, resultPtr int32) int32 {
	return WasmResultUnimplemented.Int32()
}

func proxyIncrementMetric(instance types.WasmInstance, metricId int32, offset int64) int32 {
	return WasmResultUnimplemented.Int32()
}

func proxyRecordMetric(instance types.WasmInstance, metricId int32, value int64) int32 {
	return WasmResultUnimplemented.Int32()
}

func proxyGetMetric(instance types.WasmInstance, metricId int32, resultUint64Ptr int32) int32 {
	return WasmResultUnimplemented.Int32()
}

type sharedQueue struct {
	id uint32
	lock sync.RWMutex
	queue []string
}

func (s *sharedQueue) enque(value string) WasmResult {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.queue = append(s.queue, value)

	return WasmResultOk
}

func (s *sharedQueue) deque() (string, WasmResult) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if len(s.queue) == 0 {
		return "", WasmResultEmpty
	}

	v := s.queue[0]
	s.queue = s.queue[1:]

	return v, WasmResultOk
}

type sharedQueueRegistry struct {
	lock sync.RWMutex
	nameToIDMap map[string]uint32
	m map[uint32]*sharedQueue
	queueIDGenerator uint32
}

func (s *sharedQueueRegistry) register(queueName string) uint32 {
	s.lock.Lock()
	defer s.lock.Unlock()

	if queueID, ok := s.nameToIDMap[queueName]; ok {
		return queueID
	}

	newQueueID := atomic.AddUint32(&s.queueIDGenerator, 1)
	s.nameToIDMap[queueName] = newQueueID
	s.m[newQueueID] = &sharedQueue{
		id:    newQueueID,
		queue: make([]string, 0),
	}

	return newQueueID
}

func (s *sharedQueueRegistry) resolve(queueName string) (uint32, WasmResult) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if queueID, ok := s.nameToIDMap[queueName]; ok {
		return queueID, WasmResultOk
	}

	return 0, WasmResultNotFound
}

func (s *sharedQueueRegistry) get(queueID uint32) *sharedQueue {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if queue, ok := s.m[queueID]; ok {
		return queue
	}

	return nil
}

var globalSharedQueueRegistry = &sharedQueueRegistry{
	nameToIDMap:      make(map[string]uint32),
	m:                make(map[uint32]*sharedQueue),
}

func proxyRegisterSharedQueue(instance types.WasmInstance, queueNamePtr int32, queueNameSize int32, tokenPtr int32) int32 {
	queueName, err := instance.GetMemory(uint64(queueNamePtr), uint64(queueNameSize))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}
	if len(queueName) == 0 {
		return WasmResultBadArgument.Int32()
	}

	queueID := globalSharedQueueRegistry.register(string(queueName))

	err = instance.PutUint32(uint64(tokenPtr), uint32(queueID))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}

	return WasmResultOk.Int32()
}

func proxyResolveSharedQueue(instance types.WasmInstance, queueNamePtr int32, queueNameSize int32, tokenPtr int32) int32 {
	queueName, err := instance.GetMemory(uint64(queueNamePtr), uint64(queueNameSize))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}
	if len(queueName) == 0 {
		return WasmResultBadArgument.Int32()
	}

	queueID, res := globalSharedQueueRegistry.resolve(string(queueName))
	if res != WasmResultOk {
		return res.Int32()
	}

	err = instance.PutUint32(uint64(tokenPtr), uint32(queueID))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}

	return WasmResultOk.Int32()
}

func copyIntoInstance(instance types.WasmInstance, v string, retPtr int32, retSize int32) WasmResult {
	addr, err := instance.Malloc(int32(len(v)))
	if err != nil {
		return WasmResultInvalidMemoryAccess
	}

	err = instance.PutMemory(addr, uint64(len(v)), []byte(v))
	if err != nil {
		return WasmResultInvalidMemoryAccess
	}

	err = instance.PutUint32(uint64(retPtr), uint32(addr))
	if err != nil {
		return WasmResultInvalidMemoryAccess
	}

	err = instance.PutUint32(uint64(retSize), uint32(len(v)))
	if err != nil {
		return WasmResultInvalidMemoryAccess
	}

	return WasmResultOk
}

func proxyDequeueSharedQueue(instance types.WasmInstance, token int32, dataPtr int32, dataSize int32) int32 {
	queue := globalSharedQueueRegistry.get(uint32(token))
	if queue == nil {
		return WasmResultNotFound.Int32()
	}

	v, res := queue.deque()
	if res != WasmResultOk {
		return res.Int32()
	}

	return copyIntoInstance(instance, v, dataPtr, dataSize).Int32()
}

func proxyEnqueueSharedQueue(instance types.WasmInstance, token int32, dataPtr int32, dataSize int32) int32 {
	queue := globalSharedQueueRegistry.get(uint32(token))
	if queue == nil {
		return WasmResultNotFound.Int32()
	}

	v, err := instance.GetMemory(uint64(dataPtr), uint64(dataSize))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}
	if len(v) == 0 {
		return WasmResultBadArgument.Int32()
	}

	res := queue.enque(string(v))

	return res.Int32()
}

type sharedDataItem struct {
	data string
	cas uint32
}

type sharedData struct {
	lock sync.RWMutex
	m map[string]*sharedDataItem
	cas uint32
}

func (s *sharedData) get(key string) (string, uint32, WasmResult) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if v, ok := s.m[key]; ok {
		return v.data, v.cas, WasmResultOk
	}

	return "", 0, WasmResultNotFound
}

func (s *sharedData) set(key string, value string, cas uint32) WasmResult {
	if key == "" {
		return WasmResultBadArgument
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	if v, ok := s.m[key]; ok {
		if v.cas != cas {
			return WasmResultCasMismatch
		}
		v.data = value
		v.cas = atomic.AddUint32(&s.cas, 1)
		return WasmResultOk
	}

	s.m[key] = &sharedDataItem{
		data: value,
		cas:  atomic.AddUint32(&s.cas, 1),
	}

	return WasmResultOk
}

var globalSharedData = &sharedData{
	m: make(map[string]*sharedDataItem),
}

func proxyGetSharedData(instance types.WasmInstance, keyPtr int32, keySize int32, valuePtr int32, valueSizePtr int32, casPtr int32) int32 {
	key, err := instance.GetMemory(uint64(keyPtr), uint64(keySize))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}
	if len(key) == 0 {
		return WasmResultBadArgument.Int32()
	}

	v, cas, res := globalSharedData.get(string(key))
	if res != WasmResultOk {
		return res.Int32()
	}

	addr, err := instance.Malloc(int32(len(v)))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}

	err = instance.PutMemory(addr, uint64(len(v)), []byte(v))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}

	err = instance.PutUint32(uint64(valuePtr), uint32(addr))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}

	err = instance.PutUint32(uint64(valueSizePtr), uint32(len(v)))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}

	err = instance.PutUint32(uint64(casPtr), uint32(cas))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}

	return WasmResultOk.Int32()
}

func proxySetSharedData(instance types.WasmInstance, keyPtr int32, keySize int32, valuePtr int32, valueSize int32, cas int32) int32 {
	key, err := instance.GetMemory(uint64(keyPtr), uint64(keySize))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}
	if len(key) == 0 {
		return WasmResultBadArgument.Int32()
	}

	value, err := instance.GetMemory(uint64(valuePtr), uint64(valueSize))
	if err != nil {
		return WasmResultInvalidMemoryAccess.Int32()
	}

	res := globalSharedData.set(string(key), string(value), uint32(cas))

	return res.Int32()
}
