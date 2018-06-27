/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/emicklei/go-restful"

	"configcenter/src/common"
	"configcenter/src/common/blog"
	"configcenter/src/common/metadata"
	commontypes "configcenter/src/common/types"
	"configcenter/src/common/util"
	"configcenter/src/scene_server/event_server/types"
	"configcenter/src/source_controller/common/instdata"
)

// Subscribe  Subscribe events
func (s *Service) Subscribe(req *restful.Request, resp *restful.Response) {
	pheader := req.Request.Header
	defErr := s.CCErr.CreateDefaultCCErrorIf(util.GetLanguage(pheader))

	sub := &metadata.Subscription{}
	if err := json.NewDecoder(req.Request.Body).Decode(&sub); err != nil {
		blog.Errorf("add subscription, but decode body failed, err: %v", err)
		resp.WriteError(http.StatusBadRequest, &metadata.RespError{Msg: defErr.Error(common.CCErrCommJSONUnmarshalFailed)})
		return
	}
	now := commontypes.Now()
	sub.Operator = util.GetUser(req.Request.Header)
	if sub.TimeOut <= 0 {
		sub.TimeOut = 10
	}
	if sub.ConfirmMode == metadata.ConfirmmodeHttpstatus &&
		sub.ConfirmPattern == "" {
		sub.ConfirmPattern = "200"
	}
	sub.LastTime = &now
	sub.SubscriptionForm = strings.Replace(sub.SubscriptionForm, " ", "", 0)

	count, err := instdata.GetSubscriptionCntByCondition(map[string]interface{}{"subscription_name": sub.SubscriptionName})
	if err != nil || count > 0 {
		resp.WriteError(http.StatusInternalServerError, &metadata.RespError{Msg: defErr.Error(common.CCErrCommDuplicateItem)})
		return
	}
	// save to the storage
	if _, err := instdata.CreateSubscription(sub); err != nil {
		blog.Error("create subscription failed, error:%s", err.Error())
		resp.WriteError(http.StatusInternalServerError, &metadata.RespError{Msg: defErr.Error(common.CCErrEventSubscribeInsertFailed)})
		return
	}

	// save to subscribeform in cache
	events := strings.Split(sub.SubscriptionForm, ",")
	for _, event := range events {
		if err := s.cache.SAdd(types.EventCacheSubscribeformKey+event, sub.SubscriptionID).Err(); err != nil {
			blog.Error("create subscription failed, error:%s", err.Error())
			resp.WriteError(http.StatusInternalServerError, &metadata.RespError{Msg: defErr.Error(common.CCErrEventSubscribeInsertFailed)})
			return
		}
	}

	mesg, _ := json.Marshal(&sub)
	s.cache.Publish(types.EventCacheProcessChannel, "create"+string(mesg))
	s.cache.Del(types.EventCacheDistCallBackCountPrefix + fmt.Sprint(sub.SubscriptionID))

	resp.WriteEntity(metadata.RspSubscriptionCreate{
		BaseResp: metadata.SuccessBaseResp,
		Data: struct {
			SubscriptionID int64 `json:"subscription_id"`
		}{
			SubscriptionID: sub.SubscriptionID,
		},
	})

}

func (s *Service) UnSubscribe(req *restful.Request, resp *restful.Response) {
	pheader := req.Request.Header
	defErr := s.CCErr.CreateDefaultCCErrorIf(util.GetLanguage(pheader))

	id, err := strconv.ParseInt(req.PathParameter("subscribeID"), 10, 64)
	if nil != err {
		resp.WriteError(http.StatusBadRequest, &metadata.RespError{Msg: defErr.Error(common.CCErrCommJSONUnmarshalFailed)})
		return
	}

	// query old Subscription
	sub := metadata.Subscription{}
	condiction := util.NewMapBuilder(common.BKSubscriptionIDField, id).Build()
	if err := instdata.GetOneSubscriptionByCondition(condiction, &sub); err != nil {
		blog.Error("fail to get subscription by id %v, error information is %v", id, err)
		resp.WriteError(http.StatusInternalServerError, &metadata.RespError{Msg: defErr.Error(common.CCErrEventSubscribeDeleteFailed)})
		return
	}
	// execute delete command
	if delerr := instdata.DelSubscriptionByCondition(condiction); nil != delerr {
		blog.Error("fail to delete subscription by id %v, error information is %v", id, delerr)
		resp.WriteError(http.StatusInternalServerError, &metadata.RespError{Msg: defErr.Error(common.CCErrEventSubscribeDeleteFailed)})
	}

	subID := fmt.Sprint(id)
	eventTypes := strings.Split(sub.SubscriptionForm, ",")
	for _, eventType := range eventTypes {
		eventType = strings.TrimSpace(eventType)
		if err := s.cache.SRem(types.EventCacheSubscribeformKey+eventType, subID).Err(); err != nil {
			blog.Error("delete subscription failed, error:%s", err.Error())
			resp.WriteError(http.StatusInternalServerError, &metadata.RespError{Msg: defErr.Error(common.CCErrEventSubscribeDeleteFailed)})
		}
	}

	s.cache.Del(types.EventCacheDistIDPrefix+subID,
		types.EventCacheDistQueuePrefix+subID,
		types.EventCacheDistDonePrefix+subID)

	mesg, _ := json.Marshal(&sub)
	s.cache.Publish(types.EventCacheProcessChannel, "delete"+string(mesg))

	resp.WriteEntity(metadata.NewSuccessResp(nil))
}

func (s *Service) Rebook(req *restful.Request, resp *restful.Response) {
	pheader := req.Request.Header
	defErr := s.CCErr.CreateDefaultCCErrorIf(util.GetLanguage(pheader))

	id, err := strconv.ParseInt(req.PathParameter("subscribeID"), 10, 64)
	if nil != err {
		resp.WriteError(http.StatusBadRequest, &metadata.RespError{Msg: defErr.Error(common.CCErrCommJSONUnmarshalFailed)})
		return
	}
	blog.Info("update subscription %v", id)

	sub := &metadata.Subscription{}
	if err := json.NewDecoder(req.Request.Body).Decode(&sub); err != nil {
		blog.Errorf("update subscription, but decode body failed, err: %v", err)
		resp.WriteError(http.StatusBadRequest, &metadata.RespError{Msg: defErr.Error(common.CCErrCommJSONUnmarshalFailed)})
		return
	}
	// query old Subscription
	oldsub := metadata.Subscription{}
	condiction := util.NewMapBuilder(common.BKSubscriptionIDField, id).Build()
	if err := instdata.GetOneSubscriptionByCondition(condiction, &oldsub); err != nil {
		blog.Error("fail to get subscription by id %v, error information is %v", id, err)
		resp.WriteError(http.StatusBadRequest, &metadata.RespError{Msg: defErr.Error(common.CCErrEventSubscribeUpdateFailed)})
		return
	}
	if oldsub.SubscriptionName != sub.SubscriptionName {
		count, err := instdata.GetSubscriptionCntByCondition(map[string]interface{}{"subscription_name": sub.SubscriptionName})
		if err != nil {
			blog.Errorf("get subscription count error: %v", err)
			resp.WriteError(http.StatusBadRequest, &metadata.RespError{Msg: defErr.Error(common.CCErrEventSubscribeUpdateFailed)})
			return
		}
		if count > 0 {
			blog.Error("duplicate subscription name")
			resp.WriteError(http.StatusBadRequest, &metadata.RespError{Msg: defErr.Error(common.CCErrCommDuplicateItem)})
			return
		}
	}

	sub.SubscriptionID = oldsub.SubscriptionID
	if sub.TimeOut <= 0 {
		sub.TimeOut = 10
	}
	now := commontypes.Now()
	sub.LastTime = &now
	sub.SubscriptionForm = strings.Replace(sub.SubscriptionForm, " ", "", 0)
	sub.Operator = util.GetUser(req.Request.Header)
	if updateerr := instdata.UpdateSubscriptionByCondition(sub, util.NewMapBuilder(common.BKSubscriptionIDField, id).Build()); nil != updateerr {
		blog.Error("fail update subscription by condition, error information is %s", updateerr.Error())
		resp.WriteError(http.StatusBadRequest, &metadata.RespError{Msg: defErr.Error(common.CCErrEventSubscribeUpdateFailed)})
		return
	}

	eventTypes := strings.Split(sub.SubscriptionForm, ",")
	oldeventTypes := strings.Split(oldsub.SubscriptionForm, ",")

	subs, plugs := util.CalSliceDiff(oldeventTypes, eventTypes)

	for _, eventType := range subs {
		eventType = strings.TrimSpace(eventType)
		if err := s.cache.SRem(types.EventCacheSubscribeformKey+eventType, id).Err(); err != nil {
			blog.Error("delete subscription failed, error:%s", err.Error())
			resp.WriteError(http.StatusBadRequest, &metadata.RespError{Msg: defErr.Error(common.CCErrEventSubscribeUpdateFailed)})
			return
		}
	}
	for _, event := range plugs {
		if err := s.cache.SAdd(types.EventCacheSubscribeformKey+event, sub.SubscriptionID).Err(); err != nil {
			blog.Error("create subscription failed, error:%s", err.Error())
			resp.WriteError(http.StatusBadRequest, &metadata.RespError{Msg: defErr.Error(common.CCErrEventSubscribeUpdateFailed)})
			return
		}
	}

	mesg, _ := json.Marshal(&sub)
	s.cache.Publish(types.EventCacheProcessChannel, "update"+string(mesg))

	resp.WriteEntity(metadata.NewSuccessResp(nil))
}

func (s *Service) Query(req *restful.Request, resp *restful.Response) {
	pheader := req.Request.Header
	defErr := s.CCErr.CreateDefaultCCErrorIf(util.GetLanguage(pheader))

	blog.Info("select subscription")

	var dat metadata.ParamSubscriptionSearch
	if err := json.NewDecoder(req.Request.Body).Decode(&dat); err != nil {
		blog.Errorf("search subscription, but decode body failed, err: %v", err)
		resp.WriteError(http.StatusBadRequest, &metadata.RespError{Msg: defErr.Error(common.CCErrCommJSONUnmarshalFailed)})
		return
	}

	fields := dat.Fields
	condition := dat.Condition

	skip := dat.Page.Start
	limit := dat.Page.Limit
	if limit <= 0 {
		limit = common.BKNoLimit
	}
	sort := dat.Page.Sort

	count, err := instdata.GetSubscriptionCntByCondition(condition)
	if err != nil {
		blog.Error("get host count error, input:%+v error:%v", dat, err)
		resp.WriteError(http.StatusBadRequest, &metadata.RespError{Msg: defErr.Error(common.CCErrEventSubscribeSelectFailed)})
		return
	}

	results := []metadata.Subscription{}
	blog.Debug("selector:%+v", condition)
	if selerr := instdata.GetSubscriptionByCondition(fields, condition, &results, sort, skip, limit); nil != selerr {
		blog.Error("select data failed, error information is %s, input:%v", selerr, dat)
		resp.WriteError(http.StatusBadRequest, &metadata.RespError{Msg: defErr.Error(common.CCErrEventSubscribeSelectFailed)})
		return
	}

	for index := range results {
		val := s.cache.HGetAll(types.EventCacheDistCallBackCountPrefix + fmt.Sprint(results[index].SubscriptionID)).Val()
		failue, _ := strconv.ParseInt(val["failue"], 10, 64)
		total, _ := strconv.ParseInt(val["total"], 10, 64)
		results[index].Statistics = &metadata.Statistics{
			Total:   total,
			Failure: failue,
		}
	}

	info := make(map[string]interface{})
	info["count"] = count
	info["info"] = results
	resp.WriteEntity(
		metadata.NewSuccessResp(
			metadata.RspSubscriptionSearch{
				Count: count,
				Info:  results,
			}))
}

func (s *Service) Ping(req *restful.Request, resp *restful.Response) {
	pheader := req.Request.Header
	defErr := s.CCErr.CreateDefaultCCErrorIf(util.GetLanguage(pheader))

	var dat metadata.ParamSubscriptionTestCallback
	if err := json.NewDecoder(req.Request.Body).Decode(&dat); err != nil {
		blog.Errorf("ping subscription, but decode body failed, err: %v", err)
		resp.WriteError(http.StatusBadRequest, &metadata.RespError{Msg: defErr.Error(common.CCErrCommJSONUnmarshalFailed)})
		return
	}

	callbackurl := dat.CallbackUrl
	callbackBody := dat.Data

	blog.Infof("requesting callback: %v,%s", callbackurl, callbackBody)
	callbackreq, _ := http.NewRequest("POST", callbackurl, bytes.NewBufferString(callbackBody))
	callbackResp, err := http.DefaultClient.Do(callbackreq)
	if err != nil {
		blog.Error("test distribute error:%v", err)
		resp.WriteError(http.StatusBadRequest, &metadata.RespError{Msg: defErr.Error(common.CCErrEventSubscribePingFailed)})
		return
	}
	defer callbackResp.Body.Close()

	callbackRespBody, err := ioutil.ReadAll(callbackResp.Body)
	if err != nil {
		blog.Error("test distribute error:%v", err)
	}
	result := metadata.RspSubscriptionTestCallback{}
	result.HttpStatus = callbackResp.StatusCode
	result.ResponseBody = string(callbackRespBody)

	resp.WriteEntity(metadata.NewSuccessResp(result))
}

func (s *Service) Telnet(req *restful.Request, resp *restful.Response) {
	pheader := req.Request.Header
	defErr := s.CCErr.CreateDefaultCCErrorIf(util.GetLanguage(pheader))
	var dat metadata.ParamSubscriptionTelnet
	if err := json.NewDecoder(req.Request.Body).Decode(&dat); err != nil {
		blog.Errorf("telnet subscription, but decode body failed, err: %v", err)
		resp.WriteError(http.StatusBadRequest, &metadata.RespError{Msg: defErr.Error(common.CCErrCommJSONUnmarshalFailed)})
		return
	}
	callbackurl := dat.CallbackUrl
	uri, err := util.GetDailAddress(callbackurl)
	if err != nil {
		blog.Error("telent callback error:%v", err)
		resp.WriteError(http.StatusBadRequest, &metadata.RespError{Msg: defErr.Errorf(common.CCErrCommParamsInvalid, "bk_callback_url")})
		return
	}
	blog.Infof("telnet %", uri)

	conn, err := net.Dial("tcp", uri)
	if err != nil {
		blog.Error("telent callback error:%v", err)
		resp.WriteError(http.StatusBadRequest, &metadata.RespError{Msg: defErr.Error(common.CCErrEventSubscribeTelnetFailed)})
		return
	}
	conn.Close()

	resp.WriteEntity(metadata.NewSuccessResp(nil))
}
