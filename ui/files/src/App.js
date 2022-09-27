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
          <option value="">Select a root...</option>
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
              liList.push(<li><ReqResp path={path} root={root} reqHash={reqHash} /></li>)
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

class ReqResp extends React.Component {
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
      return <span>Error: {error.message}</span>;
    } else {
      if (!isExpanded) {
        return ( <span><span onClick={this.toggleExpanded}>+ {reqHash}</span></span> )
      } else if (reqHead && respHead) {
        return (
          <span><span onClick={this.toggleExpanded}>- {reqHash}</span>
            <div>
              Request Header:<br/>
              <pre>{JSON.stringify(reqHead, null, "  ")}</pre><br/>
              <ReqRespBody meta={reqHead} root={root} path={path} hash={reqHash} type="req" />
            </div>
            <div>
              Response Header:<br/>
              <pre>{JSON.stringify(respHead, null, "  ")}</pre><br/>
              <ReqRespBody meta={respHead} root={root} path={path} hash={reqHash} type="resp" />
            </div>
          </span>
        )
      } else {
        return "Loading..."
      }
    }
  }
}

class ReqRespBody extends React.Component {
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
      return ( <span>Error: {error.message}</span> );
    } else {
      if (isEmpty) {
        return ( <span>(Empty)</span> );
      } else if (isDisplayable) {
        if (!isLoaded) {
          this.downloadFile();
          return ( <span>Loading...</span> );
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

class DiffRoots extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      isLoaded: false,
      entries: [],
      error: null,
      root1: null,
      root2: null,
    };
  }

  loadDiff() {
    const { root1, root2 } = this.props;
    this.setState({
      isLoaded: false,
    })
    fetch("/api/root/"+encodeURIComponent(root1)+"/diff?root2="+encodeURIComponent(root2))
      .then(res => res.json())
      .then(
        (result) => {
          this.setState({
            isLoaded: true,
            entries: result.entries,
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
    // detect a change in roots and reload diff on change
    const { root1, root2 } = this.props;
    if (root1 !== this.state.root1 || root2 !== this.state.root2) {
      this.setState({
        root1: root1,
        root2: root2,
      });
      this.loadDiff();
    }
    const { error, isLoaded, entries } = this.state;
    var diffs = [];
    if (error) {
      return <div>Error: {error.message}</div>;
    } else if (!isLoaded) {
      return <div>Loading...</div>;
    } else {
      // loop through each entry
      // warn on unexpected results (changed req)
      const reReq = /^(sha256:[0-9a-fA-F]{64})-req-head$/
      const reResp = /^(sha256:[0-9a-fA-F]{64})-resp-head$/
      var prevPath = ""
      Object.keys(entries).forEach(index => {
        const entry = entries[index]
        if (!entry.path || entry.path.length !== 3) {
          console.log("unexpected entry path: " + JSON.stringify(entry))
        } else if (entry.action === "changed" && entry.path[2].match(reReq)) {
          console.log("unexpected changed request: " + JSON.stringify(entry))
        } else if (entry.path[2].match(reResp)) {
          // skip path output if this matches previous path
          if (prevPath !== entry.path[1]) {
            diffs.push(<li>{entry.path[1]}</li>);
            prevPath = entry.path[1];
          }
          const ppath=entry.path.slice(0, -1)
          const reMatch = entry.path[2].match(reResp)
          // switch based on action, delete shows root1, add shows root 2, change shows both
          switch (entry.action) {
            case "deleted":
              diffs.push(<ul><li>{entry.action}: <ReqResp path={ppath} root={root1} reqHash={reMatch[1]} /></li></ul>)
              break;
            case "added":
              diffs.push(<ul><li>{entry.action}: <ReqResp path={ppath} root={root2} reqHash={reMatch[1]} /></li></ul>)
              break;
            case "changed":
              diffs.push(<ul><li>{entry.action}: <ReqResp path={ppath} root={root1} reqHash={reMatch[1]} />
                                         -&gt; <ReqResp path={ppath} root={root2} reqHash={reMatch[1]} /></li></ul>)
              break;
            default:
              console.log("unhandled action: " + entry.action)
          }
        }
      })
      return ( <ul>{ diffs }</ul> );
    }
  }
}

class DiffSelect extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      root1: "",
      root2: "",
    };
    this.handleChangeRoot1 = this.handleChangeRoot1.bind(this)
    this.handleChangeRoot2 = this.handleChangeRoot2.bind(this)
  }

  handleChangeRoot1(root) {
    this.setState({
      root1: root
    })
  }
  handleChangeRoot2(root) {
    this.setState({
      root2: root
    })
  }

  render() {
    let rootDiff = ""
    if (this.state.root1 !== "" && this.state.root2 !== "") {
      rootDiff = ( <DiffRoots root1={this.state.root1} root2={this.state.root2} /> )
    }
    return (
      // start the page with a dropdown of roots, include import/export, refresh
      // on-change update selected state
      // append a tree element for current selected state, passing the root to the tree
      <div>
        <RootList selected={this.state.root1} onChangeRoot={this.handleChangeRoot1} />
        -&gt;
        <RootList selected={this.state.root2} onChangeRoot={this.handleChangeRoot2} />
        <br/>
        { rootDiff }
      </div>
    );
  }
}


class App extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      selected: "inspect"
    };
    this.handleClick = this.handleClick.bind(this);
  }

  handleClick(e) {
    this.setState({
      selected: e.target.attributes["name"].value,
    });
  }

  render() {
    const { selected } = this.state;
    var tabContent=""
    if (selected === "inspect") {
      tabContent=<RootInspect/>;
    } else if (selected === "diff") {
      tabContent=<DiffSelect/>;
    }
    return (
      <div style={{ textAlign: 'left' }}>
      <header>
        {/* Add a set of tabs for different areas (inspect, diff, validate, link to swagger API) */}
        <span name="inspect" class={"tabBar" + (selected === "inspect" ? " selected" : "")} onClick={this.handleClick}>Inspect</span>
        <span name="diff"    class={"tabBar" + (selected === "diff" ?    " selected" : "")} onClick={this.handleClick}>Diff</span>
      </header>
      { tabContent }
    </div>
    );
  }
}

export default App;
