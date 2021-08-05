package storage

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/sudo-bmitch/reproducible-proxy/config"
	"github.com/sudo-bmitch/reproducible-proxy/storage/backing"
)

func TestStorage(t *testing.T) {
	c := config.Config{}
	c.Storage.Backing = "memory"
	mb := backing.Get(c)
	if mb == nil {
		t.Error("Failed to create the default backing")
		return
	}

	root, err := DirNew(mb)
	if err != nil {
		t.Errorf("DirNew error: %v", err)
	}

	subdir, err := root.CreateDir("subdir")
	if err != nil {
		t.Errorf("root.CreateDir error: %v", err)
	}

	subfile, err := root.CreateFile("subfile")
	if err != nil {
		t.Errorf("root.CreateFile error: %v", err)
	}

	subcomplex, err := root.CreateComplex("subcomplex")
	if err != nil {
		t.Errorf("root.CreateComplex error: %v", err)
	}

	subdirfile, err := subdir.CreateFile("subdir-file")
	if err != nil {
		t.Errorf("subdir.CreateFile error: %v", err)
	}

	scmeta := `{"meta":"value", "version":1}`
	scw, err := subcomplex.Write("meta")
	if err != nil {
		t.Errorf("subcomplex.Write error: %v", err)
	}
	l, err := scw.Write([]byte(scmeta))
	if err != nil {
		t.Errorf("scw.Write error: %v", err)
	}
	if l != len([]byte(scmeta)) {
		t.Errorf("scw.Write length mismatch: got %d, expected %d", l, len([]byte(scmeta)))
	}
	err = scw.Close()
	if err != nil {
		t.Errorf("scw.Close error: %v", err)
	}

	scdata := "Hello complex file"
	scw, err = subcomplex.Write("data")
	if err != nil {
		t.Errorf("subcomplex.Write error: %v", err)
	}
	l, err = scw.Write([]byte(scdata))
	if err != nil {
		t.Errorf("scw.Write error: %v", err)
	}
	if l != len([]byte(scdata)) {
		t.Errorf("scw.Write length mismatch: got %d, expected %d", l, len([]byte(scdata)))
	}
	err = scw.Close()
	if err != nil {
		t.Errorf("scw.Close error: %v", err)
	}

	sfdata := "Hello file in root"
	sfw, err := subfile.Write()
	if err != nil {
		t.Errorf("subfile.Write error: %v", err)
	}
	l, err = sfw.Write([]byte(sfdata))
	if err != nil {
		t.Errorf("sfw.Write error: %v", err)
	}
	if l != len([]byte(sfdata)) {
		t.Errorf("sfw.Write length mismatch: got %d, expected %d", l, len([]byte(sfdata)))
	}
	err = sfw.Close()
	if err != nil {
		t.Errorf("sfw.Close error: %v", err)
	}

	sdfdata := "Hello file in subdir"
	sdfw, err := subdirfile.Write()
	if err != nil {
		t.Errorf("subdirfile.Write error: %v", err)
	}
	l, err = sdfw.Write([]byte(sdfdata))
	if err != nil {
		t.Errorf("sdfw.Write error: %v", err)
	}
	if l != len([]byte(sdfdata)) {
		t.Errorf("sdfw.Write length mismatch: got %d, expected %d", l, len([]byte(sdfdata)))
	}
	err = sdfw.Close()
	if err != nil {
		t.Errorf("sdfw.Close error: %v", err)
	}

	rootHash, err := root.Hash()
	if err != nil {
		t.Errorf("root.Hash error: %v", err)
	}
	rootJSON, err := json.Marshal(root)
	if err != nil {
		t.Errorf("json.Marshal(root) error: %v", err)
	}
	t.Logf("root: hash %s, json %s", rootHash, rootJSON)

	sch, err := subcomplex.Hash()
	if err != nil {
		t.Errorf("subcomplex.Hash error: %v", err)
	}
	scj, err := json.Marshal(subcomplex)
	if err != nil {
		t.Errorf("json.Marshal(subcomplex) error: %v", err)
	}
	t.Logf("subcomplex: hash %s, json %s", sch, scj)

	sfh, err := subfile.Hash()
	if err != nil {
		t.Errorf("subfile.Hash error: %v", err)
	}
	sfj, err := json.Marshal(subfile)
	if err != nil {
		t.Errorf("json.Marshal(subfile) error: %v", err)
	}
	t.Logf("subfile: hash %s, json %s", sfh, sfj)

	// load a new root
	root2, err := DirLoad(mb, rootHash)
	if err != nil {
		t.Errorf("DirLoad error: %v", err)
	}

	// get various sub objects
	subdir2, err := root2.GetDir("subdir")
	if err != nil {
		t.Errorf("GetDir error: %v", err)
	}

	subfile2, err := root2.GetFile("subfile")
	if err != nil {
		t.Errorf("GetFile error: %v", err)
	}

	subcomplex2, err := root2.GetComplex("subcomplex")
	if err != nil {
		t.Errorf("GetComplex error: %v", err)
	}

	subdirfile2, err := subdir2.GetFile("subdir-file")
	if err != nil {
		t.Errorf("subdir2.GetFile error: %v", err)
	}

	scr, err := subcomplex2.Read("meta")
	if err != nil {
		t.Errorf("subcomplex2.Read error: %v", err)
	}
	sc2meta, err := ioutil.ReadAll(scr)
	if err != nil {
		t.Errorf("ioutil.ReadAll(scr) error: %v", err)
	}
	scr, err = subcomplex2.Read("data")
	if err != nil {
		t.Errorf("subcomplex2.Read error: %v", err)
	}
	sc2data, err := ioutil.ReadAll(scr)
	if err != nil {
		t.Errorf("ioutil.ReadAll(scr) error: %v", err)
	}
	sfr, err := subfile2.Read()
	if err != nil {
		t.Errorf("subfile2.Read error: %v", err)
	}
	sf2data, err := ioutil.ReadAll(sfr)
	if err != nil {
		t.Errorf("ioutil.ReadAll(sfr) error: %v", err)
	}
	sdfr, err := subdirfile2.Read()
	if err != nil {
		t.Errorf("subdirfile2.Read error: %v", err)
	}
	sdf2data, err := ioutil.ReadAll(sdfr)
	if err != nil {
		t.Errorf("ioutil.ReadAll(sdfr) error: %v", err)
	}
	root2Hash, err := root2.Hash()
	if err != nil {
		t.Errorf("root2.Hash error: %v", err)
	}
	root2JSON, err := json.Marshal(root2)
	if err != nil {
		t.Errorf("json.Marshal(root2) error: %v", err)
	}

	// compare resulting json
	if rootHash != root2Hash {
		t.Errorf("root hash mismatch, orig %s, copy %s", rootHash, root2Hash)
	}
	if bytes.Compare(rootJSON, root2JSON) != 0 {
		t.Errorf("json marshal mismatch,\n\tfirst: %s\n\tsecond: %s", rootJSON, root2JSON)
	}
	if bytes.Compare([]byte(sfdata), sf2data) != 0 {
		t.Errorf("sfdata mismatch, write %s, read %s", sfdata, sf2data)
	}
	if bytes.Compare([]byte(sdfdata), sdf2data) != 0 {
		t.Errorf("sdfdata mismatch, write %s, read %s", sdfdata, sdf2data)
	}
	if bytes.Compare([]byte(scmeta), sc2meta) != 0 {
		t.Errorf("scmeta mismatch, write %s, read %s", scmeta, sc2meta)
	}
	if bytes.Compare([]byte(scdata), sc2data) != 0 {
		t.Errorf("scdata mismatch, write %s, read %s", scdata, sc2data)
	}

	t.Logf("root2: hash %s, json %s", root2Hash, root2JSON)
}
