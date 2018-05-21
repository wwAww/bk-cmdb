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
 
package api

import (
	"configcenter/src/framework/core/output/module/inst"
)

// SetIteratorWrapper the set iterator wrapper
type SetIteratorWrapper struct {
	set inst.Iterator
}

// Next next the set
func (cli *SetIteratorWrapper) Next() (*SetWrapper, error) {

	set, err := cli.set.Next()

	return &SetWrapper{set: set}, err

}

// ForEach the foreach function
func (cli *SetIteratorWrapper) ForEach(callback func(set *SetWrapper) error) error {

	return cli.set.ForEach(func(item inst.Inst) error {
		return callback(&SetWrapper{set: item})
	})
}

// SetWrapper the set wrapper
type SetWrapper struct {
	set inst.Inst
}

// SetValue set the key value
func (cli *SetWrapper) SetValue(key string, val interface{}) error {
	return cli.set.SetValue(key, val)
}

// SetDescription set the introducrtion of the set
func (cli *SetWrapper) SetDescription(intro string) error {
	return cli.set.SetValue(fieldSetDesc, intro)
}

// SetMark set the mark of the set
func (cli *SetWrapper) SetMark(desc string) error {
	return cli.set.SetValue(fieldDescription, desc)
}

// SetEnv set the env of the set
func (cli *SetWrapper) SetEnv(env string) error {
	return cli.set.SetValue(fieldSetEnv, env)
}

// GetEnv get the env
func (cli *SetWrapper) GetEnv() (string, error) {
	vals, err := cli.set.GetValues()
	if nil != err {
		return "", err
	}
	return vals.String(fieldSetEnv), nil
}

// SetServiceStatus the service status of the set
func (cli *SetWrapper) SetServiceStatus(status string) error {
	return cli.set.SetValue(fieldServiceStatus, status)
}

// GetServiceStatus get the service status
func (cli *SetWrapper) GetServiceStatus() (string, error) {
	vals, err := cli.set.GetValues()
	if nil != err {
		return "", err
	}
	return vals.String(fieldServiceStatus), nil
}

// SetCapacity set the capacity of the set
func (cli *SetWrapper) SetCapacity(capacity int64) error {
	return cli.set.SetValue(fieldCapacity, capacity)
}

// GetCapacity get the capacity
func (cli *SetWrapper) GetCapacity() (int, error) {
	vals, err := cli.set.GetValues()
	if nil != err {
		return 0, err
	}
	return vals.Int(fieldCapacity)
}

// SetBusinessID set the business id of the set
func (cli *SetWrapper) SetBusinessID(businessID int64) error {
	if err := cli.SetParent(businessID); nil != err {
		return err
	}
	return cli.set.SetValue(fieldBusinessID, businessID)
}

// GetBusinessID get the business id
func (cli *SetWrapper) GetBusinessID() (int, error) {
	vals, err := cli.set.GetValues()
	if nil != err {
		return 0, err
	}
	return vals.Int(fieldBusinessID)
}

// SetSupplierAccount set the supplier account code of the set
func (cli *SetWrapper) SetSupplierAccount(supplierAccount string) error {
	return cli.set.SetValue(fieldSupplierAccount, supplierAccount)
}

// GetSupplierAccount get the supplier account
func (cli *SetWrapper) GetSupplierAccount() (string, error) {
	vals, err := cli.set.GetValues()
	if nil != err {
		return "", err
	}
	return vals.String(fieldSupplierAccount), nil
}

// GetID get the set id
func (cli *SetWrapper) GetID() (int, error) {
	vals, err := cli.set.GetValues()
	if nil != err {
		return 0, err
	}
	return vals.Int(fieldSetID)
}

// SetParent set the parent id of the set
func (cli *SetWrapper) SetParent(parentInstID int64) error {
	return cli.set.SetValue(fieldParentID, parentInstID)
}

// SetName the name of the set
func (cli *SetWrapper) SetName(name string) error {
	return cli.set.SetValue(fieldSetName, name)
}

// GetName get the set name
func (cli *SetWrapper) GetName() (string, error) {
	vals, err := cli.set.GetValues()
	if nil != err {
		return "", err
	}
	return vals.String(fieldSetName), nil
}

// Save save the data
func (cli *SetWrapper) Save() error {
	return cli.set.Save()
}
