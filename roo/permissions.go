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
	Rune int32
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
	fmt.Println(kv)
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

func (kvs *KvService) authorize(ctx context.Context, request *Request) bool {
	//TODO
	//Cache example
	// USER ONLY KEY
	// dpachla:/org/folder1/folder2/wb/wbid1234/pivot/pivotid13455/write
	// Successful Resource-Depth/Entity pair
	// /org/workspace/folder1/github_committers

	parts := strings.Split(request.Resource.Val, "/")
	path := ""
	for _, part := range parts {
		path = "/" + part
		if kvs.checkPermission(ctx, getPermissionString(
			&request.User,
			&TypedString{
				Val:  path,
				Rune: request.Resource.Rune,
			},
			&request.Action,
			&request.Right,
			false,
		)) {
			rlog.Infof("[GET] Permission succeeded on resource: %#U %s for user: %s passed: %s\n", request.Resource.Rune, request.Resource.Val, request.User, request.Resource.Val)
			return true
		}
		for _, e := range request.Entities {
			if e.Val == request.User.Val {
				continue
			}
			if kvs.checkPermission(ctx, getPermissionString(
				&e,
				&TypedString{
					Val:  path,
					Rune: request.Resource.Rune,
				},
				&request.Action,
				&request.Right,
				false,
			)) {
				rlog.Infof("[GET] Permission succeeded on resource: %#U %s for user: %s passed: %s\n", request.Resource.Rune, request.Resource.Val, request.User, request.Resource.Val)
				return true
			}
		}
	}

	return false
}

// Examples
// (p) permission (e) entity user/group (c) entity context, resource, ex. folder, integration, organization-unit
// KEY: p:{e}:sourcetable_users:{c}:/org/folder1 VALUE: 00000000101010101010 (ex. edit, comment)
// KEY: p:{e}:sourcetable_users:{c}:/org/folder1:{a}:push_prs, VALUE: 1 (RESERVED - IS ACTION SPECIAL TYPE)
func (kvs *KvService) permiss(ctx context.Context, permissions []Permisson) error {
	cctx, cancel := context.WithTimeout(ctx, time.Duration(12*time.Second))
	defer cancel()
	values := make([]*KVData, len(permissions))
	//Convert permissions to a batch then write
	for i := 0; i < len(permissions); i++ {
		var v []byte
		k := fmt.Sprintf("p:e:%s:%c:%s",
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

func (kvs *KvService) prune(ctx context.Context) (interface{}, error) {
	//TODO Prune cache
	return nil, fmt.Errorf("Method not implemented (prune)")
}
