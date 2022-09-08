import React from 'react';

class RootList extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      error: null,
      isLoaded: false,
      roots: []
    };
    this.handleChange = this.handleChange.bind(this);
  }

  componentDidMount() {
    fetch("/api/root")
      .then(res => res.json())
      .then(
        (result) => {
          this.setState({
            isLoaded: true,
            roots: result
          });
        },
        (error) => {
          this.setState({
            isLoaded: true,
            error
          });
        }
      )
  }

  handleChange(e) {
    this.props.onChangeRoot(e.target.value);
  }

  render() {
    const { error, isLoaded, roots } = this.state;
    if (error) {
      return <div>Error: {error.message}</div>;
    } else if (!isLoaded) {
      return <div>Loading...</div>;
    } else {
      return (
        // start the page with a dropdown of roots, include import/export, refresh
        // on-change update selected state
        // append a tree element for current selected state, passing the root to the tree
        <select value={this.props.selected} onChange={this.handleChange} >
          <option>Select a root...</option>
          {roots.map(root => (
            <option><pre>{root}</pre></option>
          ))}
        </select>
      );
    }
  }
}

class TreeDir extends React.Component {
  constructor(props) {
    super(props);
    // props should have: path, root
    this.state = {
      error: null,
      isLoaded: false,
      isExpanded: false,
      entries: [],
      path: (props.path || [])
    };
    this.toggleExpanded = this.toggleExpanded.bind(this);
  }

  componentDidMount() {
    if (!this.state.path.length) {
      this.toggleExpanded()
    } else if (this.state.isExpanded) {
      this.loadEntries()
    }
  }

  loadEntries() {
    const { root } = this.props
    const { path } = this.state
    let url = "/api/root/"+encodeURIComponent(root)+"/dir"
    for (let i = 0; i < path.length; i++) {
      url += (i === 0 ? "?" : "&") + "path=" + encodeURIComponent(path[i])
    }
    fetch(url)
      .then(res => res.json())
      .then(
        (result) => {
          this.setState({
            isLoaded: true,
            entries: result
          });
        },
        (error) => {
          this.setState({
            isLoaded: true,
            error
          });
        }
      )
  }

  toggleExpanded() {
    const { isExpanded, isLoaded } = this.state
    if (!isExpanded && !isLoaded ) {
      this.loadEntries()
    }
    this.setState(state => ({
      isExpanded: !state.isExpanded
    }))
  }

  render() {
    const { error, isLoaded, isExpanded, entries, path } = this.state;
    const { root } = this.props
    if (error) {
      return <div>Error: {error.message}</div>;
    } else {
      let header=""
      if (!path.length) {
        if (!isLoaded) {
          header = "Loading..."
        }
      } else {
        let prefix = "-"
        let name = path[path.length-1]
        if (!isExpanded) {
          // collapsed list
          prefix = "+"
        } else if (!isLoaded) {
          // loading list
          prefix = "*"
        }
        header = ( <div onClick={this.toggleExpanded}>{prefix} {name}</div> )
      }
      let showEntries = ""
      if (isExpanded && isLoaded) {
        let liList = []
        const re = /^(sha256:[0-9a-fA-F]{64})-req-head$/
        Object.keys(entries).forEach(name => {
          if (entries[name].kind === "dir") {
            liList.push(<li><TreeDir path={path.concat(name)} root={root} /></li>)
          } else if (entries[name].kind === "file") {
            const reMatch = name.match(re)
            if (reMatch) {
              const reqHash = reMatch[1]
              liList.push(<li><TreeReq path={path} root={root} reqHash={reqHash} /></li>)
            }
          }
        })
        showEntries = ( <ul>{liList}</ul> )
      }

      // start the page with a dropdown of roots, include import/export, refresh
      // on-change update selected state
      // append a tree element for current selected state, passing the root to the tree
      return ( <div> {header} {showEntries} </div> )
    }
  }
}

class TreeReq extends React.Component {
  constructor(props) {
    super(props);
    // props should have: reqHash, path, root
    this.state = {
      error: null,
      isExpanded: false,
      reqHead: null,
      reqBody: null,
      respHead: null,
      respBody: null,
      path: (props.path || [])
    };
    this.toggleExpanded = this.toggleExpanded.bind(this)
    this.loadHead = this.loadHead.bind(this)
  }

  componentDidMount() {
    if (this.state.isExpanded) {
      this.loadHead()
    }
  }

  loadHead() {
    const { reqHash, root } = this.props
    const { path } = this.state
    let url = "/api/root/"+encodeURIComponent(root)+"/file"
    for (let i = 0; i < path.length; i++) {
      url += (i === 0 ? "?" : "&") + "path=" + encodeURIComponent(path[i])
    }
    const urlReqHead = url + "&path=" + encodeURIComponent(reqHash + "-req-head")
    fetch(urlReqHead)
      .then(res => res.json())
      .then(
        (result) => {
          this.setState({
            reqHead: result
          });
        },
        (error) => {
          this.setState({
            error
          });
        }
      )
    const urlRespHead = url + "&path=" + encodeURIComponent(reqHash + "-resp-head")
    fetch(urlRespHead)
      .then(res => res.json())
      .then(
        (result) => {
          this.setState({
            respHead: result
          });
        },
        (error) => {
          this.setState({
            error
          });
        }
      )
  }

  toggleExpanded() {
    const { isExpanded, reqHead, respHead } = this.state
    if (!isExpanded && (!reqHead || !respHead) ) {
      this.loadHead()
    }
    this.setState(state => ({
      isExpanded: !state.isExpanded
    }))
  }

  render() {
    const { error, isExpanded, path, reqHead, respHead } = this.state;
    const { reqHash, root } = this.props;
    if (error) {
      return <div>Error: {error.message}</div>;
    } else {
      if (!isExpanded) {
        return ( <div><span onClick={this.toggleExpanded}>+ {reqHash}</span></div> )
      } else if (reqHead && respHead) {
        return (
          <div><span onClick={this.toggleExpanded}>- {reqHash}</span><br/>
            Request Header:<br/>
            <pre>{JSON.stringify(reqHead, null, "  ")}</pre><br/>
            <TreeFile meta={reqHead} root={root} path={path} hash={reqHash} type="req" />
            Response Header:<br/>
            <pre>{JSON.stringify(respHead, null, "  ")}</pre>
            <TreeFile meta={respHead} root={root} path={path} hash={reqHash} type="resp" />
          </div>
        )
      } else {
        return "Loading..."
      }
    }
  }
}

class TreeFile extends React.Component {
  constructor(props) {
    super(props);
    // props should have: meta, root, path, hash, type
    this.state = {
      ct: "",
      error: null,
      isLoaded: false,
      isEmpty: false,
      isDisplayable: false,
    };
    if (this.props.meta.Headers && this.props.meta.Headers["Content-Type"] && this.props.meta.Headers["Content-Type"].length > 0) {
      this.state.ct = this.props.meta.Headers["Content-Type"][0]
    }
    this.state.urlFile = "/api/root/"+encodeURIComponent(this.props.root)+"/file?ct="+encodeURIComponent(this.state.ct)
    this.state.urlResp = "/api/root/"+encodeURIComponent(this.props.root)+"/resp?hash="+encodeURIComponent(this.props.hash)
    for (let i = 0; i < this.props.path.length; i++) {
      this.state.urlFile += "&path=" + encodeURIComponent(this.props.path[i])
      this.state.urlResp += "&path=" + encodeURIComponent(this.props.path[i])
    }
    this.state.urlFile += "&path=" + encodeURIComponent(this.props.hash + "-" + this.props.type + "-body")
  }

  componentDidMount() {
    this.setState(this.checkDisplayable(this.props.meta))
    if (this.state.isDisplayable && !this.state.isLoaded) {
      this.downloadFile()
    }
  }

  checkDisplayable(meta) {
    const { ct } = this.state;
    if (meta.ContentLen === 0) {
      return {isEmpty: true}
    }
    const allowedCT = ["application/http", "application/json", "application/xml"]
    if (!ct.startsWith("text/") && !allowedCT.includes(ct)) {
      // reject unknown content types
      return {}
    } else if (meta.ContentLen > 100000) {
      // too large
      return {}
    }
    // known media type and small enough
    return {isDisplayable: true}
  }

  downloadFile() {
    const { urlFile } = this.state
    fetch(urlFile)
      .then(res => res.text())
      .then(
        (result) => {
          this.setState({
            isLoaded: true,
            content: result
          });
        },
        (error) => {
          this.setState({
            isLoaded: true,
            error
          });
        }
      )
  }

  render() {
    const { content, error, isDisplayable, isEmpty, isLoaded, urlResp } = this.state;
    if (error) {
      return ( <div>Error: {error.message}</div> );
    } else {
      if (isEmpty) {
        return ( <div>(Empty)</div> );
      } else if (isDisplayable) {
        if (!isLoaded) {
          return ( <div>Loading...</div> );
        } else {
          return ( <pre>{content}</pre> );
        }
      } else {
        // non-displayable uses a download link
        return ( <a style={{display: "table-cell"}} href={urlResp} target="_blank" rel="noreferrer">Download</a> )
      }
    }
  }
}

class RootInspect extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      root: ""
    };
    this.handleChangeRoot = this.handleChangeRoot.bind(this)
  }

  handleChangeRoot(root) {
    this.setState({
      root: root
    })
  }

  render() {
    let rootTree = ""
    if (this.state.root !== "") {
      rootTree = ( <TreeDir key={this.state.root} root={this.state.root} path={[]} /> )
    }
    return (
      // start the page with a dropdown of roots, include import/export, refresh
      // on-change update selected state
      // append a tree element for current selected state, passing the root to the tree
      <div>
        <RootList selected={this.state.root} onChangeRoot={this.handleChangeRoot} />
        { rootTree }
      </div>
    );
  }
}

function App() {
  return (
    <div style={{ textAlign: 'left' }}>
      <header>
        {/* Add a set of tabs for different areas (inspect, diff, validate, link to swagger API) */}
        Inspect
      </header>
      {/* default to inspect */}
      <RootInspect/>
    </div>
  );
}

export default App;
