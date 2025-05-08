/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package service

import "net/http"

// interface definition for DataStore_Provider, the below methods will be implemented by the implementation providers ( for now local, & ADE)
type DataStoreProvider interface {
	GetDataURL() string
	SetAuthHeader(req *http.Request)
}

// DataStoreProvider default interface implementation that reads Victoria database
type DefaultDataStoreProvider struct {
	localDataStoreUrl string
}

func NewDefaultDataStoreProvider(localDataStoreUrl string) *DefaultDataStoreProvider {
	defaultDataStoreProvider := new(DefaultDataStoreProvider)
	defaultDataStoreProvider.localDataStoreUrl = localDataStoreUrl
	return defaultDataStoreProvider
}

func (ds *DefaultDataStoreProvider) GetDataURL() string {
	return ds.localDataStoreUrl
}

func (ds *DefaultDataStoreProvider) SetAuthHeader(req *http.Request) {
	// Do nothing
}
