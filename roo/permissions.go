package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// RIGHT
const (
	ACTION         = 1 << iota //Reserved bit for action (not a bitwise)
	LIST                       //Ex. see file in folder
	OPEN                       //Open the document & see properties
	COMMENT                    //Comment in sidebar
	APPEND                     //Non destructive changes
	COPY                       //Clone a file to my directory
	EDIT                       //Destructive changes
	SHARE_INTERNAL             //Share to someone inside the original org
	SHARE_EXTERNAL             //Share to someone outside the original org
	MOVE                       //Move the document
	DESTROY
	ADMIN
	IMPEROSONATE
	OWNER
)

const (
	USER        = "U"
	FILE        = "F"
	ENTITY      = "E"
	INTEGRATION = "I"
)

type TypedString struct {
	Val  string
	Rune rune
}

type Permisson struct {
	Entity  TypedString //entity is a union of group & user
	Context TypedString //context for permissioned user, ex. f:/org/folder1
	Action  TypedString //an action not covered in the rights above
	//ACTION TAKES PRECEDENCE,
	//Right is IGNORED if exists.
	//Ex. push_pr (not in rights)
	Right  []byte //READ, OPEN, DESTROY etc.
	Delete bool
}

type Request struct {
	User     TypedString   //me
	Entities []TypedString //resources I'm in
	Resource TypedString   //resource requested (slug), ex. nested file, folder, integration, organization to be permissioned
	Action   TypedString   //an action not covered in the rights above
	//ACTION TAKES PRECEDENCE,
	//Right is IGNORED if exists.
	//Ex. push_pr (not in rights)
	Right []byte //READ, OPEN, DESTROY etc.
}

// Examples
// (p) permission (e) entity user/group (c) entity context, resource, ex. folder, integration, organization-unit
// KEY: p:{e}:sourcetable_users:{c}:/org/folder1 VALUE: 00000000101010101010 (ex. edit, comment)
// KEY: p:{e}:sourcetable_users:{c}:/org/folder1:{a}:push_prs, VALUE: 1 (RESERVED - IS ACTION SPECIAL TYPE)
func getPermissionString(entity *TypedString, context *TypedString, action *TypedString, right *[]byte, delete bool) *KVData {
	var v []byte
	k := fmt.Sprintf("p:e:%s:%c:%s",
		entity.Val,
		context.Rune,
		context.Val,
	)
	if len(action.Val) > 0 {
		k = k + fmt.Sprintf(":%c:%s", action.Rune, action.Val)
		v = []byte{1}
	} else {
		v = *right
	}
	kv := &KVData{
		Key: k,
		Val: v,
	}
	if delete {
		kv.Val = nil
	}
	//fmt.Println("PERMISSION CHECK", kv)
	return kv
}

func (kvs *KvService) checkPermission(ctx context.Context, kv *KVData) bool {
	cctx, cancel := context.WithTimeout(ctx, time.Duration(12*time.Second))
	defer cancel()
	result, err := kvs.nh.SyncRead(cctx, kvs.AppConfig.Cluster.ShardID, kv.Key)
	if result == nil {
		return false
	} else if err != nil {
		rlog.Errorf("SyncRead returned error (checkPermission) %v\n", err)
		return false
	}
	if bytes, ok := result.([]byte); ok {
		if len(bytes) == 0 {
			return false
		} else if len(bytes) == 1 && bytes[0]&1 > 0 {
			//Action matches
			return true
		} else {
			//Check every byte
			if kvs.AppConfig.PermissonCheckExact {
				return bitwiseAnd(bytes, kv.Val)
			} else {
				return bitwiseGreaterOrEqual(kv.Val, bytes)
			}
		}
	} else {
		return false
	}
}

func (kvs *KvService) authorize(ctx context.Context, request *Request) (bool, error) {
	//TODO: Clean/Validate/Lowercase/Remove ":" from request

	//First try the cache
	//Returns {entity_type_char}:{entity}:{context_type_char}:{context}
	c := kvs.hit(ctx, request)
	if len(c) > 0 {
		//URGENT: First check we are still the/member of entity
		found := false
		prevContext := strings.Split(c, ":")
		if len(prevContext) == 4 {
			//WARNING: ASSUMING MEMBER/GROUP HAS A UNIQUE KEY ACROSS BOTH
			if prevContext[1] == request.User.Val {
				found = true
			} else {
				for _, e := range request.Entities {
					if prevContext[1] == e.Val {
						found = true
					}
				}
			}
			if found {
				var v []byte
				k := fmt.Sprintf("p:%s",
					c,
				)
				if len(request.Action.Val) > 0 {
					k = k + fmt.Sprintf(":%c:%s", request.Action.Rune, request.Action.Val)
					v = []byte{1}
				} else {
					v = request.Right
				}
				if kvs.checkPermission(ctx, &KVData{
					Key: k,
					Val: v,
				}) {
					//TODO: Maybe cache the cache
					rlog.Infof("[GET] Permission succeeded on resource: %#U %s for user: %s passed: %s\n", request.Resource.Rune, request.Resource.Val, request.User, request.Resource.Val)
					return true, nil
				}
			}

		}

	}

	//TODO - much more rigorous testing is needed
	//Perhaps org name is a priority (maybe match chars to path,entity and resource first)
	//Maybe some instance we leave user til end (not first)
	parts := strings.Split(request.Resource.Val, "/")
	path := ""
	for idx, part := range parts {
		if idx > 0 {
			path = path + "/" + part //TODO: We assume minimum of a root object
		}
		pString := getPermissionString(
			&request.User,
			&TypedString{
				Val:  path,
				Rune: request.Resource.Rune,
			},
			&request.Action,
			&request.Right,
			false,
		)
		if kvs.checkPermission(ctx, pString) {
			kvs.cache(ctx, request, &request.User, &TypedString{
				Rune: request.Resource.Rune,
				Val:  path,
			})
			rlog.Infof("[GET] Permission succeeded on resource: %#U %s for user: %s passed: %s\n", request.Resource.Rune, request.Resource.Val, request.User, request.Resource.Val)
			return true, nil
		}
		for _, e := range request.Entities {
			if e.Val == request.User.Val {
				continue
			}
			pString = getPermissionString(
				&e,
				&TypedString{
					Val:  path,
					Rune: request.Resource.Rune,
				},
				&request.Action,
				&request.Right,
				false,
			)
			if kvs.checkPermission(ctx, pString) {
				kvs.cache(ctx, request, &e, &TypedString{
					Rune: request.Resource.Rune,
					Val:  path,
				})
				if kvs.AppConfig.Debug {
					rlog.Debugf("[GET] Permission succeeded on resource: %#U %s for: %v\n", request.Resource.Rune, request.Resource.Val, request)
				}
				return true, nil
			}
		}
	}

	return false, nil
}

// Examples
// (p) permission (e) entity user/group (c) entity context, resource, ex. folder, integration, organization-unit
// KEY: p:{e}:sourcetable_users:{c}:/org/folder1 VALUE: 00000000101010101010 (ex. edit, comment)
// KEY: p:{e}:sourcetable_users:{c}:/org/folder1:{a}:push_prs, VALUE: 1 (RESERVED - IS ACTION SPECIAL TYPE)
// NOTE: DO NOT USE NAMES IN CASE THEY ARE RENAMED, USE IDS
func (kvs *KvService) permiss(ctx context.Context, permissions []Permisson) error {
	cctx, cancel := context.WithTimeout(ctx, time.Duration(12*time.Second))
	defer cancel()
	values := make([]*KVData, len(permissions))
	//Convert permissions to a batch then write
	for i := 0; i < len(permissions); i++ {
		var v []byte
		k := fmt.Sprintf("p:e:%s:%c:%s", //NOTE: We force all users/groups to be a union of both and unique
			permissions[i].Entity.Val,
			permissions[i].Context.Rune,
			permissions[i].Context.Val,
		)
		if len(permissions[i].Action.Val) > 0 {
			k = k + fmt.Sprintf(":%c:%s", permissions[i].Action.Rune, permissions[i].Action.Val)
			v = []byte{1}
		} else {
			v = permissions[i].Right
		}
		values[i] = &KVData{
			Key: k,
			Val: v,
		}
		if permissions[i].Delete {
			values[i].Val = nil
		}
	}
	kvb, err := json.Marshal(&KVBatch{Action: PUT, Batch: values})
	cs := kvs.nh.GetNoOPSession(kvs.AppConfig.Cluster.ShardID)
	if err != nil {
		return err
	}
	if _, err := kvs.nh.SyncPropose(cctx, cs, kvb); err != nil {
		return err
	}
	return nil

}

func getCacheKey(request *Request) string {
	k := fmt.Sprintf("c:%s:%s",
		request.User.Val,
		request.Resource.Val,
	)
	if len(request.Action.Val) > 0 {
		k = k + fmt.Sprintf(":%c:%s", request.Action.Rune, request.Action.Val)
	} else {
		k = k + fmt.Sprintf(":%s", request.Right)
	}
	return k
}

// Cache
// KEY: {user}:{resource}, VALUE: {entity}:{context}
// Example requestedby:requestedresource:requestedprivilege
// Example c:dpachla:/org/folder1/folder2/wb/wbid1234/pivot/pivotid13455:{write|01010101} , e:sourcetable_users:{c}:/org
func (kvs *KvService) cache(ctx context.Context, request *Request, successfulEntity *TypedString, successfulContext *TypedString) error {
	if kvs.AppConfig.CacheSeconds <= 1 {
		return nil
	}
	//First add cache, item then add expiry
	cctx, cancel := context.WithTimeout(ctx, time.Duration(12*time.Second))
	defer cancel()

	k := getCacheKey(request)

	v := fmt.Sprintf("e:%s:%c:%s", //Note default classification of all users & groups as entity "e"
		successfulEntity.Val,
		successfulContext.Rune,
		successfulContext.Val,
	)

	if kv, err := json.Marshal(&KVData{Key: k, Val: []byte(v)}); err != nil {
		return err
	} else {
		cs := kvs.nh.GetNoOPSession(kvs.AppConfig.Cluster.ShardID)
		if _, err := kvs.nh.SyncPropose(cctx, cs, kv); err != nil {
			return err
		}
		kvs.ttl(ctx, k)
	}

	return nil
}

// Add keys for expiry for cleaning out
func (kvs *KvService) ttl(ctx context.Context, key string) error {
	cctx, cancel := context.WithTimeout(ctx, time.Duration(12*time.Second))
	defer cancel()

	k := fmt.Sprintf("ttl:%015d:%s",
		time.Now().Unix()+int64(kvs.AppConfig.CacheSeconds),
		randString(12),
	)

	if kv, err := json.Marshal(&KVData{Key: k, Val: []byte(key)}); err != nil {
		return err
	} else {
		cs := kvs.nh.GetNoOPSession(kvs.AppConfig.Cluster.ShardID)
		if _, err := kvs.nh.SyncPropose(cctx, cs, kv); err != nil {
			return err
		}

	}
	return nil
}

// Check the cache, use if found
// Returns {entity_type_char}:{entity}:{context_type_char}:{context}
func (kvs *KvService) hit(ctx context.Context, request *Request) string {
	cctx, cancel := context.WithTimeout(ctx, time.Duration(12*time.Second))
	defer cancel()
	k := getCacheKey(request)
	result, err := kvs.nh.SyncRead(cctx, kvs.AppConfig.Cluster.ShardID, k)
	if result == nil {
		return "" //MISS!
	} else if err != nil {
		return "" //MISS!
	}
	if bytes, ok := result.([]byte); ok && len(bytes) > 0 {
		return fmt.Sprintf("%s", bytes)
	}
	return "" //MISS!
}

// Clear out the cache
func (kvs *KvService) prune(ctx context.Context) error {
	cctx, cancel := context.WithTimeout(ctx, time.Duration(30*time.Second))
	defer cancel()

	k := fmt.Sprintf("ttl:%015d",
		time.Now().Unix(),
	)

	if kv, err := json.Marshal(&KVAction{Action: REVERSE_SCAN, Data: &KVData{Key: k, Val: []byte("ttl:")}}); err != nil {
		return err
	} else {
		if result, err := kvs.nh.SyncRead(cctx, kvs.AppConfig.Cluster.ShardID, kv); err != nil {
			return err
		} else {
			items := result.(map[string][]byte)
			cs := kvs.nh.GetNoOPSession(kvs.AppConfig.Cluster.ShardID)
			for i, b := range items {

				//Not setting a value will delete from keystore
				if ttl, err := json.Marshal(&KVData{
					Key: i,
				}); err != nil {
					return err
				} else {
					if _, err := kvs.nh.SyncPropose(cctx, cs, ttl); err != nil {
						return err
					}
					if cached, err := json.Marshal(&KVData{
						Key: string(b),
					}); err != nil {
						return err
					} else {
						if _, err := kvs.nh.SyncPropose(cctx, cs, cached); err != nil {
							return err
						}
					}
				}

			}
		}

	}
	return nil
}

// Clear out the cache for user
func (kvs *KvService) revoke(ctx context.Context, user string) error {
	cctx, cancel := context.WithTimeout(ctx, time.Duration(60*time.Second))
	defer cancel()

	if kv, err := json.Marshal(&KVAction{Action: SCAN, Data: &KVData{Key: fmt.Sprintf("c:%s", user)}}); err != nil {
		return err
	} else {
		if result, err := kvs.nh.SyncRead(cctx, kvs.AppConfig.Cluster.ShardID, kv); err != nil {
			return err
		} else {
			items := result.(map[string][]byte)
			cs := kvs.nh.GetNoOPSession(kvs.AppConfig.Cluster.ShardID)
			for i := range items {
				//Not setting a value will delete from keystore
				if c, err := json.Marshal(&KVData{
					Key: i,
				}); err != nil {
					return err
				} else {
					if _, err := kvs.nh.SyncPropose(cctx, cs, c); err != nil {
						continue
					}
				}

			}
		}

	}
	return nil
}
