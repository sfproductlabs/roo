package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	IMPEROSONATE
	ADMIN
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
	Entity   TypedString
	Context  TypedString //context for permissioned user, ex. f:/org/folder1
	Resource TypedString //nested file, folder, integration, organization to be permissioned
	Action   TypedString //an action not covered in the rights above
	//ACTION TAKES PRECEDENCE,
	//Right is IGNORED if exists.
	//Ex. push_pr (not in rights)
	Right  []byte //READ, OPEN, DESTROY etc.
	Delete bool
}

type Request struct {
	User     TypedString   //me
	Entities []TypedString //resources I'm in
	Resource TypedString   //resource requested (slug)
	Action   TypedString   //an action not covered in the rights above
	//ACTION TAKES PRECEDENCE,
	//Right is IGNORED if exists.
	//Ex. push_pr (not in rights)
	Right []byte //READ, OPEN, DESTROY etc.
}

func (kvs *KvService) authorize(ctx context.Context, request *Request) (interface{}, error) {
	result := false
	cctx, cancel := context.WithTimeout(ctx, time.Duration(12*time.Second))
	defer cancel()
	//TODO
	//Cache example
	// USER ONLY KEY
	// dpachla:/org/folder1/folder2/wb/wbid1234/pivot/pivotid13455/write
	// Successful Resource-Depth/Entity pair
	// /org/workspace/folder1/github_committers

	_, err := kvs.nh.SyncRead(cctx, kvs.AppConfig.Cluster.ShardID, request.Action)
	// Resource: /org/folder1/folder2/wb/wbid1234/pivot/pivotid13455
	// Action: read
	if err != nil {
		rlog.Errorf("SyncRead returned error %v\n", err)
		return nil, err
	} else {
		rlog.Infof("[GET] Permission on resource: %#U %s for user: %s passed: %s\n", request.Resource.Rune, request.Resource.Val, request.User, result)
		return result, nil
	}

	return false, fmt.Errorf("Method not implemented (authorize)")
}

// Examples
// (p) permission (e) entity user/group (c) entity context (r) resource, ex. folder, integration, organization-unit
// KEY: p:{e}:sourcetable_users:{c}:/org:{r}:/org/folder1 VALUE: 00000000101010101010 (ex. edit, comment)
// p:{e}:sourcetable_users:{c}:/org:{r}:/org/folder1:{ACTION}, 1
func (kvs *KvService) permiss(ctx context.Context, permissions []Permisson) error {
	cctx, cancel := context.WithTimeout(ctx, time.Duration(12*time.Second))
	defer cancel()
	values := make([]*KVData, len(permissions))
	//Convert permissions to a batch then write
	for i := 0; i < len(permissions); i++ {
		var v []byte
		k := fmt.Sprintf("p:e:%s:%c:%s:%c:%s",
			permissions[i].Entity.Val,
			permissions[i].Context.Rune,
			permissions[i].Context.Val,
			permissions[i].Resource.Rune,
			permissions[i].Resource.Val,
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
