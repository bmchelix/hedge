/*******************************************************************************
 * Copyright 2018 Redis Labs Inc.
 * (c) Copyright 2020-2025 BMC Software, Inc.
 *
 * Contributors: BMC Software, Inc. - BMC Helix Edge
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License
 * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
 * or implied. See the License for the specific language governing permissions and limitations under
 * the License.
 *******************************************************************************/
package redis

import (
	db2 "hedge/common/db"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gomodule/redigo/redis"

	"github.com/google/uuid"
)

func GetObjectById(conn redis.Conn, id string, unmarshal unmarshalFunc, out interface{}) error {
	object, err := redis.Bytes(conn.Do("GET", id))
	if err == redis.ErrNil {
		return db2.ErrNotFound
	} else if err != nil {
		return err
	}

	return unmarshal(object, out)
}

// TODO: Discuss this with Andre as a possibly replacement for GetObjectByHash
// 1.) key/value seems clearer to me than hash/field for equivalent concepts. However the latter
//
//	may be more consistently used in the Redis community. If so, revert.
//
// 2.) Not sure the custom "unmarshal" function is necessary when no domain logic is encapsulated
//
//	within the Redis-based models. If the signatures of the Redis models are the same as contract
//	then just use contract. However we have the capability to specialize the Redis models as
//	needed now should a future requirement arise.
func GetObjectByKey(conn redis.Conn, key string, value string, out interface{}) error {
	id, err := redis.String(conn.Do("HGET", key, value))
	if err == redis.ErrNil {
		return db2.ErrNotFound
	} else if err != nil {
		return err
	}

	object, err := redis.Bytes(conn.Do("GET", id))
	if err != nil {
		return err
	}
	return json.Unmarshal(object, out)
}

func GetObjectByHash(conn redis.Conn, hash string, field string, unmarshal unmarshalFunc, out interface{}) error {
	id, err := redis.String(conn.Do("HGET", hash, field))
	if err == redis.ErrNil {
		return db2.ErrNotFound
	} else if err != nil {
		return err
	}

	object, err := redis.Bytes(conn.Do("GET", id))
	if err != nil {
		return err
	}

	return unmarshal(object, out)
}

func GetObjectsByValue(conn redis.Conn, v string) ([][]byte, error) {
	ids, err := redis.Values(conn.Do("SMEMBERS", v))
	if err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		return nil, nil
	}

	objects, err := redis.ByteSlices(conn.Do("MGET", ids...))
	if err != nil {
		return nil, err
	}

	return objects, nil
}

func GetObjectsByValues(conn redis.Conn, vals ...string) ([][]byte, error) {
	args := redis.Args{}
	for _, v := range vals {
		args = args.Add(v)
	}
	ids, err := redis.Values(conn.Do("SINTER", args...))
	if err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		return nil, nil
	}

	objects, err := redis.ByteSlices(conn.Do("MGET", ids...))
	if err != nil {
		return nil, err
	}

	return objects, nil
}

// GetObjectsByRange retrieves the entries for keys enumerated in a sorted set.
// The entries are retrieved in the sorted set order.
func GetObjectsByRange(conn redis.Conn, key string, start, end int) ([][]byte, error) {
	return GetObjectsBySomeRange(conn, "ZRANGE", key, start, end)
}

// GetObjectsByRevRange retrieves the entries for keys enumerated in a sorted set.
// The entries are retrieved in the reverse sorted set order.
func GetObjectsByRevRange(conn redis.Conn, key string, start int, end int) ([][]byte, error) {
	return GetObjectsBySomeRange(conn, "ZREVRANGE", key, start, end)
}

// GetObjectsBySomeRange retrieves the entries for keys enumerated in a sorted set using the specified Redis range
// command (i.e. RANGE, REVRANGE). The entries are retrieved in the order specified by the supplied Redis command.
func GetObjectsBySomeRange(conn redis.Conn, command string, key string, start int, end int) ([][]byte, error) {
	ids, err := redis.Values(conn.Do(command, key, start, end))
	if err != nil && err != redis.ErrNil {
		return nil, err
	}

	var result [][]byte
	if len(ids) > 0 {
		result, err = redis.ByteSlices(conn.Do("MGET", ids...))
		if err != nil {
			return nil, err
		}
	}

	var objects [][]byte
	for _, obj := range result {
		if obj != nil {
			objects = append(objects, obj)
		}
	}

	return objects, nil

}

// Return objects by a score from a zset
// if limit is 0, all are returned
// if end is negative, it is considered as positive infinity
func GetObjectsByRangeFilter(conn redis.Conn, key string, filter string, start, end int) ([][]byte, error) {
	ids, err := redis.Values(conn.Do("ZRANGE", key, start, end))
	if err != nil && err != redis.ErrNil {
		return nil, err
	}

	var objects [][]byte
	// https://github.com/golang/go/wiki/SliceTricks#filtering-without-allocating
	fids := ids[:0]
	if len(ids) > 0 {
		for _, id := range ids {
			err := conn.Send("ZSCORE", filter, id)
			if err != nil {
				return nil, err
			}
		}
		scores, err := redis.Strings(conn.Do(""))
		if err != nil {
			return nil, err
		}

		for i, score := range scores {
			if score != "" {
				fids = append(fids, ids[i])
			}
		}

		objects, err = redis.ByteSlices(conn.Do("MGET", fids...))
		if err != nil {
			return nil, err
		}
	}
	return objects, nil
}

func GetObjectsByScore(conn redis.Conn, key string, start, end int64, limit int) ([][]byte, error) {
	args := []interface{}{key, start}
	if end < 0 {
		args = append(args, "+inf")
	} else {
		args = append(args, end)
	}
	if limit != 0 {
		args = append(args, "LIMIT")
		args = append(args, 0)
		args = append(args, limit)
	}
	ids, err := redis.Values(conn.Do("ZRANGEBYSCORE", args...))
	if err != nil && err != redis.ErrNil {
		return nil, err
	}

	var objects [][]byte
	if len(ids) > 0 {
		objects, err = redis.ByteSlices(conn.Do("MGET", ids...))
		if err != nil {
			return nil, err
		}
	}
	return objects, nil
}

// addObject is responsible for setting the object's primary record and then sending the appropriate
// follow-on commands as provided by the caller.

// Transactions are managed outside of this function.
/*
func AddObject(data []byte, adder models.Adder, id string, conn redis.Conn) {
	_ = conn.Send("SET", id, data)

	for _, cmd := range adder.Add() {
		switch cmd.Command {
		case "ZADD":
			_ = conn.Send(cmd.Command, cmd.Hash, cmd.Rank, cmd.Key)
		case "SADD":
			_ = conn.Send(cmd.Command, cmd.Hash, cmd.Key)
		case "HSET":
			_ = conn.Send(cmd.Command, cmd.Hash, cmd.Key, cmd.Value)
		}
	}
}

// deleteObject is responsible for removing the object's primary record and then sending the appropriate
// follow-on commands as provided by the caller.
//
// Transactions are managed outside of this function.
func DeleteObject(remover models.Remover, id string, conn redis.Conn) {
	_ = conn.Send("DEL", id)

	for _, cmd := range remover.Remove() {
		switch cmd.Command {
		case "ZREM":
			fallthrough
		case "SREM":
			fallthrough
		case "HDEL":
			_ = conn.Send(cmd.Command, cmd.Hash, cmd.Key)
		}
	}
}
*/

func GetUnionObjectsByValues(conn redis.Conn, vals ...string) ([][]byte, error) {
	args := redis.Args{}
	for _, v := range vals {
		args = args.Add(v)
	}
	ids, err := redis.Values(conn.Do("SUNION", args...))
	if err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		return nil, nil
	}

	objects, err := redis.ByteSlices(conn.Do("MGET", ids...))
	if err != nil {
		return nil, err
	}

	return objects, nil
}

func GetObjectsByValuesSorted(conn redis.Conn, limit int, vals ...string) ([][]byte, error) {
	args := redis.Args{}

	cacheSet := uuid.New().String()

	args = append(args, cacheSet)
	args = append(args, strconv.Itoa(len(vals)))
	for _, val := range vals {
		args = append(args, val)
	}

	_, err := conn.Do("ZINTERSTORE", args...)
	if err != nil {
		return nil, err
	}

	ids, err := redis.Values(conn.Do("ZREVRANGE", cacheSet, 0, -1))
	if err != nil {
		return nil, err
	}

	if limit < 0 || limit > len(ids) {
		limit = len(ids)
	}
	objects, err := redis.ByteSlices(conn.Do("MGET", ids[0:limit]...))
	if err != nil {
		return nil, err
	}

	// clean up now unused ZINTERSTORE cache
	_, err = redis.Int(conn.Do("DEL", cacheSet))
	if err != nil {
		return nil, err
	}

	return objects, nil
}

func ValidateKeyExists(conn redis.Conn, key string) error {
	count, err := redis.Int(conn.Do("EXISTS", key))
	if err != nil {
		return err
	}

	if count == 1 {
		return nil
	}
	return db2.ErrNotFound
}

// GetObjectsByPattern retrieves all objects matching a given key pattern.
func GetObjectsByPattern(conn redis.Conn, pattern string) ([][]byte, error) {
	var cursor int64
	var keys []string

	for {
		// SCAN the keys based on the pattern
		reply, err := redis.Values(conn.Do("SCAN", cursor, "MATCH", pattern, "COUNT", 10))
		if err != nil {
			return nil, fmt.Errorf("error scanning keys with pattern %s: %v", pattern, err)
		}

		// Unmarshal cursor and keys
		cursor, _ = redis.Int64(reply[0], nil)
		foundKeys, _ := redis.Strings(reply[1], nil)
		keys = append(keys, foundKeys...)

		// Stop if we've iterated over all keys
		if cursor == 0 {
			break
		}
	}

	// Return empty if no keys found
	if len(keys) == 0 {
		return nil, nil
	}

	// Fetch objects by the found keys
	objects, err := redis.ByteSlices(conn.Do("MGET", redis.Args{}.AddFlat(keys)...))
	if err != nil {
		return nil, fmt.Errorf("error fetching objects for keys %v: %v", keys, err)
	}

	return objects, nil
}
