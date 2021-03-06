//
// Copyright (c) 2015 The heketi Authors
//
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
//

package glusterfs

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/heketi/tests"
	"github.com/heketi/utils"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func init() {
	// turn off logging
	logger.SetLevel(utils.LEVEL_NOLOG)
}

func TestClusterCreate(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// ClusterCreate JSON Request
	request := []byte(`{
    }`)

	// Post nothing
	r, err := http.Post(ts.URL+"/clusters", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusCreated)

	// Read JSON
	var msg ClusterInfoResponse
	err = utils.GetJsonFromResponse(r, &msg)
	tests.Assert(t, err == nil)

	// Test JSON
	tests.Assert(t, len(msg.Nodes) == 0)
	tests.Assert(t, len(msg.Volumes) == 0)

	// Check that the data on the database is recorded correctly
	var entry ClusterEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		return entry.Unmarshal(
			tx.Bucket([]byte(BOLTDB_BUCKET_CLUSTER)).
				Get([]byte(msg.Id)))
	})
	tests.Assert(t, err == nil)

	// Make sure they entries are euqal
	tests.Assert(t, entry.Info.Id == msg.Id)
	tests.Assert(t, len(entry.Info.Volumes) == 0)
	tests.Assert(t, len(entry.Info.Nodes) == 0)
}

func TestClusterList(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Save some objects in the database
	numclusters := 5
	err := app.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BOLTDB_BUCKET_CLUSTER))
		if b == nil {
			return errors.New("Unable to open bucket")
		}

		for i := 0; i < numclusters; i++ {
			var entry ClusterEntry

			entry.Info.Id = fmt.Sprintf("%v", 5000+i)
			buffer, err := entry.Marshal()
			if err != nil {
				return err
			}

			err = b.Put([]byte(entry.Info.Id), buffer)
			if err != nil {
				return err
			}
		}

		return nil

	})
	tests.Assert(t, err == nil)

	// Now that we have some data in the database, we can
	// make a request for the clutser list
	r, err := http.Get(ts.URL + "/clusters")
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

	// Read response
	var msg ClusterListResponse
	err = utils.GetJsonFromResponse(r, &msg)
	tests.Assert(t, err == nil)

	// Thanks to BoltDB they come back in order
	mockid := 5000 // This is the mock id value we set above
	for _, id := range msg.Clusters {
		tests.Assert(t, id == fmt.Sprintf("%v", mockid))
		mockid++
	}
}

func TestClusterInfoIdNotFound(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Now that we have some data in the database, we can
	// make a request for the clutser list
	r, err := http.Get(ts.URL + "/clusters/12345")
	tests.Assert(t, r.StatusCode == http.StatusNotFound)
	tests.Assert(t, err == nil)
}

func TestClusterInfo(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Create a new ClusterInfo
	entry := NewClusterEntry()
	entry.Info.Id = "123"
	for _, node := range []string{"a1", "a2", "a3"} {
		entry.NodeAdd(node)
	}
	for _, vol := range []string{"b1", "b2", "b3"} {
		entry.VolumeAdd(vol)
	}

	// Save the info in the database
	err := app.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BOLTDB_BUCKET_CLUSTER))
		if b == nil {
			return errors.New("Unable to open bucket")
		}

		buffer, err := entry.Marshal()
		if err != nil {
			return err
		}

		err = b.Put([]byte(entry.Info.Id), buffer)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil)

	// Now that we have some data in the database, we can
	// make a request for the clutser list
	r, err := http.Get(ts.URL + "/clusters/" + "123")
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

	// Read response
	var msg ClusterInfoResponse
	err = utils.GetJsonFromResponse(r, &msg)
	tests.Assert(t, err == nil)

	// Check values are equal
	tests.Assert(t, entry.Info.Id == msg.Id)
	tests.Assert(t, entry.Info.Volumes[0] == msg.Volumes[0])
	tests.Assert(t, entry.Info.Volumes[1] == msg.Volumes[1])
	tests.Assert(t, entry.Info.Volumes[2] == msg.Volumes[2])
	tests.Assert(t, entry.Info.Nodes[0] == msg.Nodes[0])
	tests.Assert(t, entry.Info.Nodes[1] == msg.Nodes[1])
	tests.Assert(t, entry.Info.Nodes[2] == msg.Nodes[2])
}

func TestClusterDeleteBadId(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Delete cluster with no elements
	req, err := http.NewRequest("DELETE", ts.URL+"/clusters/12345", nil)
	tests.Assert(t, err == nil)
	r, err := http.DefaultClient.Do(req)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusNotFound)
}

func TestClusterDelete(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Create an entry with volumes and nodes
	entries := make([]*ClusterEntry, 0)
	entry := NewClusterEntry()
	entry.Info.Id = "a1"
	for _, node := range []string{"a1", "a2", "a3"} {
		entry.NodeAdd(node)
	}
	for _, vol := range []string{"b1", "b2", "b3"} {
		entry.VolumeAdd(vol)
	}
	entries = append(entries, entry)

	// Create an entry with only volumes
	entry = NewClusterEntry()
	entry.Info.Id = "a2"
	for _, vol := range []string{"b1", "b2", "b3"} {
		entry.VolumeAdd(vol)
	}
	entries = append(entries, entry)

	// Create an entry with only nodes
	entry = NewClusterEntry()
	entry.Info.Id = "a3"
	for _, node := range []string{"a1", "a2", "a3"} {
		entry.NodeAdd(node)
	}
	entries = append(entries, entry)

	// Create an empty entry
	entry = NewClusterEntry()
	entry.Info.Id = "000"
	entries = append(entries, entry)

	// Save the info in the database
	err := app.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BOLTDB_BUCKET_CLUSTER))
		if b == nil {
			return errors.New("Unable to open bucket")
		}

		for _, entry := range entries {
			buffer, err := entry.Marshal()
			if err != nil {
				return err
			}

			err = b.Put([]byte(entry.Info.Id), buffer)
			if err != nil {
				return err
			}
		}

		return nil

	})
	tests.Assert(t, err == nil)

	// Check that we cannot delete a cluster with elements
	req, err := http.NewRequest("DELETE", ts.URL+"/clusters/"+"a1", nil)
	tests.Assert(t, err == nil)
	r, err := http.DefaultClient.Do(req)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusConflict)

	// Check that we cannot delete a cluster with volumes
	req, err = http.NewRequest("DELETE", ts.URL+"/clusters/"+"a2", nil)
	tests.Assert(t, err == nil)
	r, err = http.DefaultClient.Do(req)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusConflict)

	// Check that we cannot delete a cluster with nodes
	req, err = http.NewRequest("DELETE", ts.URL+"/clusters/"+"a3", nil)
	tests.Assert(t, err == nil)
	r, err = http.DefaultClient.Do(req)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusConflict)

	// Delete cluster with no elements
	req, err = http.NewRequest("DELETE", ts.URL+"/clusters/"+"000", nil)
	tests.Assert(t, err == nil)
	r, err = http.DefaultClient.Do(req)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusOK)

	// Check database still has a1,a2, and a3, but not '000'
	err = app.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BOLTDB_BUCKET_CLUSTER))
		if b == nil {
			return errors.New("Unable to open bucket")
		}

		// Check that the ids are still in the database
		for _, id := range []string{"a1", "a2", "a3"} {
			buffer := b.Get([]byte(id))
			if buffer == nil {
				return errors.New(fmt.Sprintf("Id %v not found", id))
			}
		}

		// Check that the id 000 is no longer in the database
		buffer := b.Get([]byte("000"))
		if buffer != nil {
			return errors.New(fmt.Sprintf("Id 000 still in database and was deleted"))
		}

		return nil

	})
	tests.Assert(t, err == nil, err)

}
