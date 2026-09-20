package nodered

import (
	"strconv"
	"strings"
)

// Everything Node-RED puts on a node besides its configuration.
var structuralKeys = map[string]bool{"id": true, "type": true, "z": true, "g": true, "name": true, "x": true, "y": true, "wires": true, "d": true, "info": true, "l": true}

var unitMs = map[string]float64{
	"milliseconds": 1, "ms": 1, "seconds": 1000, "s": 1000, "second": 1000, "minutes": 60000, "min": 60000, "minute": 60000,
	"hours": 3600000, "hr": 3600000, "hour": 3600000, "day": 86400000, "days": 86400000,
}

var msUnit = map[string]float64{"second": 1000, "minute": 60000, "hour": 3600000, "day": 86400000}

// convertConfig maps a Node-RED node's properties to a Go-RED config. The
// second result is false for node types Go-RED does not know; their raw
// properties are kept so nothing is lost.
func convertConfig(typ string, raw Raw, w *warnings) (map[string]interface{}, bool) {
	c := map[string]interface{}{}
	switch typ {
	case "inject":
		payload, ptype := str(raw["payload"]), str(raw["payloadType"])
		msg := map[string]interface{}{}
		switch ptype {
		case "date":
			w.add("inject: payload type \"timestamp\" is not supported; the payload was left empty")
			msg["payload"] = ""
		case "num":
			msg["payload"] = num(payload)
		case "bool":
			msg["payload"] = payload == "true"
		case "json":
			msg["payload"] = payload
		default:
			msg["payload"] = payload
		}
		if topic := str(raw["topic"]); topic != "" {
			c["topic"] = topic
		}
		c["payload"] = msg
		if repeat := num(raw["repeat"]); repeat > 0 {
			c["interval"] = repeat * 1000
		}
		c["injectOnce"] = boolean(raw["once"])
	case "debug":
		complete := str(raw["complete"])
		switch complete {
		case "true":
			c["output"] = "full"
		case "false", "payload", "":
			c["output"] = "payload"
		default:
			w.add("debug: only msg.payload or the complete message can be shown; %q was replaced by msg.payload", "msg."+complete)
			c["output"] = "payload"
		}
		c["outputToConsole"] = boolean(raw["console"])
		if active, ok := raw["active"]; ok {
			c["enabled"] = boolean(active)
		}
	case "function":
		c["code"] = str(raw["func"])
		c["useMsg"] = true
		if outputs := num(raw["outputs"]); outputs > 1 {
			w.add("function: multiple outputs are not supported; only the first output is wired")
		}
	case "switch":
		c["property"] = ref(propertyType(str(raw["propertyType"]), w, "switch"), str(raw["property"]))
		c["checkAll"] = str(raw["checkall"]) != "false"
		var rules []interface{}
		items, _ := raw["rules"].([]interface{})
		for _, item := range items {
			rule, _ := item.(map[string]interface{})
			op := str(rule["t"])
			if !switchOperators[op] {
				// Kept in place so the outputs after it still line up with
				// their wires; the node reports the operator at deploy.
				w.add("switch: rule operator %q is not supported; edit the rule before deploying", op)
			}
			converted := map[string]interface{}{"operator": op}
			if _, ok := rule["v"]; ok {
				converted["value"] = typed(valueType(str(rule["vt"]), w), str(rule["v"]))
			}
			if _, ok := rule["v2"]; ok {
				converted["value2"] = typed(valueType(str(rule["v2t"]), w), str(rule["v2"]))
			}
			rules = append(rules, converted)
		}
		if rules == nil {
			rules = []interface{}{}
		}
		c["rules"] = rules
	case "change":
		var rules []interface{}
		items, _ := raw["rules"].([]interface{})
		for _, item := range items {
			rule, _ := item.(map[string]interface{})
			action := str(rule["t"])
			converted := map[string]interface{}{
				"action": action,
				"target": ref(propertyType(str(rule["pt"]), w, "change"), str(rule["p"])),
			}
			switch action {
			case "set":
				converted["value"] = typed(valueType(str(rule["tot"]), w), str(rule["to"]))
			case "change":
				converted["from"] = str(rule["from"])
				converted["to"] = str(rule["to"])
				converted["fromRegex"] = str(rule["fromt"]) == "re"
			case "move":
				converted["moveTo"] = ref(propertyType(str(rule["tot"]), w, "change"), str(rule["to"]))
			case "delete":
			default:
				w.add("change: rule type %q is not supported and was dropped", action)
				continue
			}
			rules = append(rules, converted)
		}
		if rules == nil {
			rules = []interface{}{}
		}
		c["rules"] = rules
	case "template":
		c["field"] = firstNonEmpty(str(raw["field"]), "payload")
		c["syntax"] = firstNonEmpty(str(raw["syntax"]), "mustache")
		c["template"] = str(raw["template"])
		c["output"] = firstNonEmpty(str(raw["output"]), "str")
	case "delay":
		switch str(raw["pauseType"]) {
		case "rate", "timed":
			c["mode"] = "rate"
		case "delay", "":
			c["mode"] = "delay"
		default:
			w.add("delay: mode %q is not supported; a fixed delay was used", str(raw["pauseType"]))
			c["mode"] = "delay"
		}
		c["delayMs"] = num(raw["timeout"]) * unit(str(raw["timeoutUnits"]), unitMs, 1000)
		c["rateLimit"] = num(raw["rate"])
		c["rateIntervalMs"] = firstPositive(num(raw["nbRateUnits"]), 1) * unit(str(raw["rateUnits"]), msUnit, 1000)
		c["dropIntermediate"] = boolean(raw["drop"])
	case "trigger":
		c["firstPayload"] = triggerPayload(str(raw["op1type"]), str(raw["op1"]), w)
		c["secondPayload"] = triggerPayload(str(raw["op2type"]), str(raw["op2"]), w)
		c["delayMs"] = num(raw["duration"]) * unit(str(raw["units"]), unitMs, 1)
	case "exec":
		c["command"] = str(raw["command"])
		c["args"] = toInterfaces(strings.Fields(str(raw["append"])))
		addpay := str(raw["addpay"])
		c["appendPayload"] = addpay != "" && addpay != "false"
		if timer := num(raw["timer"]); timer > 0 {
			c["timeoutMs"] = timer * 1000
		}
	case "range":
		c["property"] = ref("msg", firstNonEmpty(str(raw["property"]), "payload"))
		action := str(raw["action"])
		if action == "drop" || action == "" {
			if action == "drop" {
				w.add("range: \"drop\" is not supported; values are scaled instead")
			}
			action = "scale"
		}
		c["action"] = action
		c["minin"], c["maxin"], c["minout"], c["maxout"] = num(raw["minin"]), num(raw["maxin"]), num(raw["minout"]), num(raw["maxout"])
		c["round"] = boolean(raw["round"])
	case "rbe":
		mode := str(raw["func"])
		switch mode {
		case "rbe", "rbei":
			c["mode"] = "rbe"
		case "deadband", "deadbandEq":
			c["mode"] = "deadband"
		default:
			w.add("rbe: mode %q is not supported; \"block unless value changes\" was used", mode)
			c["mode"] = "rbe"
		}
		c["gap"] = num(strings.TrimSuffix(str(raw["gap"]), "%"))
		c["property"] = ref("msg", firstNonEmpty(str(raw["property"]), "payload"))
		c["separateTopics"] = str(raw["septopics"]) != "false"
	case "batch":
		mode := str(raw["mode"])
		if mode != "count" && mode != "interval" {
			w.add("batch: mode %q is not supported; \"count\" was used", mode)
			mode = "count"
		}
		c["mode"] = mode
		c["count"] = num(raw["count"])
		c["intervalMs"] = num(raw["timeout"]) * 1000
	case "split":
		c["property"] = ref("msg", "payload")
		c["mode"] = ""
		if str(raw["spltType"]) == "str" || str(raw["spltType"]) == "" {
			c["separator"] = strings.NewReplacer(`\n`, "\n", `\t`, "\t").Replace(str(raw["splt"]))
		}
	case "join", "junction", "link in", "xml", "yaml":
	case "link out":
		c["links"] = toInterfaces(stringList(raw["links"]))
	case "catch", "status", "complete":
		if scope, ok := raw["scope"].([]interface{}); ok {
			c["scope"] = toInterfaces(stringList(scope))
		} else {
			c["scope"] = []interface{}{}
		}
	case "comment":
		c["text"] = str(raw["info"])
	case "sort":
		c["property"] = ref("msg", firstNonEmpty(str(raw["target"]), "payload"))
		c["descending"] = str(raw["order"]) == "descending"
	case "csv":
		c["delimiter"] = firstNonEmpty(str(raw["sep"]), ",")
		c["hasHeaderRow"] = str(raw["hdrin"]) == "true" || num(raw["hdrin"]) > 0
		c["columns"] = toInterfaces(splitCSVList(str(raw["temp"])))
	case "html":
		c["selector"] = str(raw["tag"])
		c["ret"] = firstNonEmpty(str(raw["ret"]), "html")
	case "json":
		c["pretty"] = boolean(raw["pretty"])
	case "file":
		c["filename"] = typed(fileNameType(str(raw["filenameType"])), str(raw["filename"]))
		switch str(raw["overwriteFile"]) {
		case "true":
			c["action"] = "overwrite"
		case "delete":
			c["action"] = "delete"
		default:
			c["action"] = "append"
		}
		c["appendNewline"] = boolean(raw["appendNewline"])
		c["createDir"] = boolean(raw["createDir"])
		if str(raw["encoding"]) == "base64" {
			c["encoding"] = "base64"
		} else {
			c["encoding"] = "utf8"
		}
	case "file in":
		c["filename"] = typed(fileNameType(str(raw["filenameType"])), str(raw["filename"]))
		format := str(raw["format"])
		if format == "stream" {
			w.add("file in: \"stream\" is not supported; the file is read as a whole")
			format = ""
		}
		c["format"] = format
		c["allProps"] = boolean(raw["allProps"])
	case "watch":
		c["files"] = str(raw["files"])
		c["recursive"] = boolean(raw["recursive"])
	case "http in":
		c["method"] = strings.ToUpper(firstNonEmpty(str(raw["method"]), "get"))
		c["path"] = str(raw["url"])
	case "http request":
		method := strings.ToUpper(str(raw["method"]))
		if method == "" || method == "USE" {
			if method == "USE" {
				w.add("http request: the method from msg.method is not supported; GET was used")
			}
			method = "GET"
		}
		c["method"] = method
		c["url"] = typed("str", str(raw["url"]))
		if tls := str(raw["tls"]); tls != "" {
			c["tls"] = tls
		}
		if proxy := str(raw["proxy"]); proxy != "" {
			c["proxy"] = proxy
		}
		headers := map[string]interface{}{}
		if list, ok := raw["headers"].([]interface{}); ok {
			for _, item := range list {
				h, _ := item.(map[string]interface{})
				key := firstNonEmpty(str(h["keyValue"]), str(h["keyType"]))
				if key != "" && key != "other" {
					headers[key] = firstNonEmpty(str(h["valueValue"]), str(h["valueType"]))
				}
			}
		}
		c["headers"] = headers
	case "http response":
		if code := num(raw["statusCode"]); code > 0 {
			c["statusCode"] = code
		}
	case "mqtt in":
		c["broker"] = str(raw["broker"])
		c["topic"] = str(raw["topic"])
		c["qos"] = num(raw["qos"])
	case "mqtt out":
		c["broker"] = str(raw["broker"])
		c["topic"] = str(raw["topic"])
		c["qos"] = num(raw["qos"])
		c["retain"] = boolean(raw["retain"])
	case "mqtt-broker":
		host := str(raw["broker"])
		port := firstNonEmpty(str(raw["port"]), "1883")
		if !strings.Contains(host, "://") {
			host = "tcp://" + host
		}
		c["url"] = host + ":" + port
		c["clientId"] = str(raw["clientid"])
		c["keepAliveSec"] = firstPositive(num(raw["keepalive"]), 60)
		c["cleanSession"] = str(raw["cleansession"]) != "false"
		c["useTLS"] = boolean(raw["usetls"])
	case "tcp in":
		c["server"] = str(raw["server"]) != "client"
		c["host"] = str(raw["host"])
		c["port"] = num(raw["port"])
		c["datatype"] = bufferType(str(raw["datatype"]))
		c["splitLines"] = str(raw["newline"]) != ""
	case "tcp out":
		c["host"] = str(raw["host"])
		c["port"] = num(raw["port"])
	case "tcp request":
		c["host"] = str(raw["server"])
		c["port"] = num(raw["port"])
		c["datatype"] = "buffer"
		if tout := num(raw["tout"]); tout > 0 {
			c["timeoutMs"] = tout
		}
	case "udp in":
		c["port"] = num(raw["port"])
		c["datatype"] = bufferType(str(raw["datatype"]))
	case "udp out":
		c["host"] = str(raw["addr"])
		c["port"] = num(raw["port"])
	case "websocket in", "websocket out":
		c["server"] = firstNonEmpty(str(raw["server"]), str(raw["client"]))
	case "websocket-listener":
		c["path"] = str(raw["path"])
	case "websocket-client":
		c["url"] = str(raw["path"])
	case "tls-config":
		c["certType"] = "files"
		c["cert"] = str(raw["cert"])
		c["key"] = str(raw["key"])
		c["ca"] = str(raw["ca"])
		c["servername"] = str(raw["servername"])
		c["verifyServerCert"] = str(raw["verifyservercert"]) != "false"
	case "http proxy":
		c["url"] = str(raw["url"])
		c["noProxy"] = toInterfaces(stringList(raw["noproxy"]))
	default:
		for k, v := range raw {
			if !structuralKeys[k] {
				c[k] = v
			}
		}
		return c, false
	}
	return c, true
}

// exportConfig maps a Go-RED config back to Node-RED properties.
func exportConfig(typ string, c map[string]interface{}) Raw {
	out := Raw{}
	switch typ {
	case "inject":
		msg, _ := c["payload"].(map[string]interface{})
		payload := msg["payload"]
		switch v := payload.(type) {
		case float64:
			out["payload"], out["payloadType"] = str(v), "num"
		case bool:
			out["payload"], out["payloadType"] = strconv.FormatBool(v), "bool"
		case nil:
			out["payload"], out["payloadType"] = "", "str"
		default:
			out["payload"], out["payloadType"] = str(v), "str"
		}
		out["topic"] = str(c["topic"])
		if interval := num(c["interval"]); interval > 0 {
			out["repeat"] = str(interval / 1000)
		} else {
			out["repeat"] = ""
		}
		out["once"] = boolean(c["injectOnce"])
		out["crontab"] = ""
	case "debug":
		if str(c["output"]) == "full" {
			out["complete"] = "true"
		} else {
			out["complete"] = "payload"
		}
		out["console"] = boolean(c["outputToConsole"])
		out["tosidebar"] = true
		if enabled, ok := c["enabled"]; ok {
			out["active"] = boolean(enabled)
		} else {
			out["active"] = true
		}
	case "function":
		out["func"] = str(c["code"])
		out["outputs"] = 1
	case "switch":
		pt, path := refOf(c["property"])
		out["property"], out["propertyType"] = path, pt
		out["checkall"] = strconv.FormatBool(c["checkAll"] != false)
		var rules []interface{}
		items, _ := c["rules"].([]interface{})
		for _, item := range items {
			rule, _ := item.(map[string]interface{})
			exported := map[string]interface{}{"t": str(rule["operator"])}
			if v, ok := rule["value"]; ok {
				vt, value := typedOf(v)
				exported["v"], exported["vt"] = value, vt
			}
			if v, ok := rule["value2"]; ok {
				vt, value := typedOf(v)
				exported["v2"], exported["v2t"] = value, vt
			}
			rules = append(rules, exported)
		}
		if rules == nil {
			rules = []interface{}{}
		}
		out["rules"] = rules
		out["outputs"] = len(rules)
	case "change":
		var rules []interface{}
		items, _ := c["rules"].([]interface{})
		for _, item := range items {
			rule, _ := item.(map[string]interface{})
			action := str(rule["action"])
			pt, path := refOf(rule["target"])
			exported := map[string]interface{}{"t": action, "p": path, "pt": pt}
			switch action {
			case "set":
				vt, value := typedOf(rule["value"])
				exported["to"], exported["tot"] = value, vt
			case "change":
				exported["from"] = str(rule["from"])
				exported["to"] = str(rule["to"])
				if boolean(rule["fromRegex"]) {
					exported["fromt"] = "re"
				} else {
					exported["fromt"] = "str"
				}
				exported["tot"] = "str"
			case "move":
				mt, mpath := refOf(rule["moveTo"])
				exported["to"], exported["tot"] = mpath, mt
			}
			rules = append(rules, exported)
		}
		if rules == nil {
			rules = []interface{}{}
		}
		out["rules"] = rules
	case "template":
		out["field"] = str(c["field"])
		out["fieldType"] = "msg"
		out["syntax"] = str(c["syntax"])
		out["template"] = str(c["template"])
		out["output"] = str(c["output"])
	case "delay":
		out["pauseType"] = firstNonEmpty(str(c["mode"]), "delay")
		out["timeout"] = str(num(c["delayMs"]) / 1000)
		out["timeoutUnits"] = "seconds"
		out["rate"] = str(num(c["rateLimit"]))
		out["nbRateUnits"] = str(num(c["rateIntervalMs"]) / 1000)
		out["rateUnits"] = "second"
		out["drop"] = boolean(c["dropIntermediate"])
	case "trigger":
		out["op1type"], out["op1"] = exportTriggerPayload(c["firstPayload"])
		out["op2type"], out["op2"] = exportTriggerPayload(c["secondPayload"])
		out["duration"] = str(num(c["delayMs"]))
		out["units"] = "ms"
	case "exec":
		out["command"] = str(c["command"])
		out["append"] = strings.Join(stringList(c["args"]), " ")
		if boolean(c["appendPayload"]) {
			out["addpay"] = "payload"
		} else {
			out["addpay"] = ""
		}
		out["timer"] = str(num(c["timeoutMs"]) / 1000)
	case "range":
		_, path := refOf(c["property"])
		out["property"] = path
		out["action"] = str(c["action"])
		out["minin"], out["maxin"], out["minout"], out["maxout"] = str(c["minin"]), str(c["maxin"]), str(c["minout"]), str(c["maxout"])
		out["round"] = boolean(c["round"])
	case "rbe":
		out["func"] = firstNonEmpty(str(c["mode"]), "rbe")
		out["gap"] = str(c["gap"])
		_, path := refOf(c["property"])
		out["property"] = path
		out["septopics"] = boolean(c["separateTopics"])
	case "batch":
		out["mode"] = str(c["mode"])
		out["count"] = str(num(c["count"]))
		out["timeout"] = str(num(c["intervalMs"]) / 1000)
	case "split":
		out["splt"] = strings.NewReplacer("\n", `\n`, "\t", `\t`).Replace(str(c["separator"]))
		out["spltType"] = "str"
	case "link out":
		out["links"] = toInterfaces(stringList(c["links"]))
		out["mode"] = "link"
	case "catch", "status", "complete":
		scope := stringList(c["scope"])
		if len(scope) == 0 {
			out["scope"] = nil
		} else {
			out["scope"] = toInterfaces(scope)
		}
	case "comment":
		out["info"] = str(c["text"])
	case "sort":
		_, path := refOf(c["property"])
		out["target"], out["targetType"] = path, "msg"
		if boolean(c["descending"]) {
			out["order"] = "descending"
		} else {
			out["order"] = "ascending"
		}
	case "csv":
		out["sep"] = str(c["delimiter"])
		if boolean(c["hasHeaderRow"]) {
			out["hdrin"] = "1"
		} else {
			out["hdrin"] = ""
		}
		out["temp"] = strings.Join(stringList(c["columns"]), ",")
	case "html":
		out["tag"] = str(c["selector"])
		out["ret"] = str(c["ret"])
	case "json":
		out["pretty"] = boolean(c["pretty"])
	case "file":
		vt, value := typedOf(c["filename"])
		out["filename"], out["filenameType"] = value, vt
		switch str(c["action"]) {
		case "overwrite":
			out["overwriteFile"] = "true"
		case "delete":
			out["overwriteFile"] = "delete"
		default:
			out["overwriteFile"] = "false"
		}
		out["appendNewline"] = boolean(c["appendNewline"])
		out["createDir"] = boolean(c["createDir"])
		out["encoding"] = firstNonEmpty(str(c["encoding"]), "none")
	case "file in":
		vt, value := typedOf(c["filename"])
		out["filename"], out["filenameType"] = value, vt
		out["format"] = str(c["format"])
		out["allProps"] = boolean(c["allProps"])
	case "watch":
		out["files"] = str(c["files"])
		out["recursive"] = boolean(c["recursive"])
	case "http in":
		out["method"] = strings.ToLower(str(c["method"]))
		out["url"] = str(c["path"])
	case "http request":
		out["method"] = str(c["method"])
		_, url := typedOf(c["url"])
		out["url"] = url
		out["ret"] = "txt"
		out["tls"] = str(c["tls"])
		out["proxy"] = str(c["proxy"])
		var headers []interface{}
		if m, ok := c["headers"].(map[string]interface{}); ok {
			for _, key := range sortedStringKeys(m) {
				headers = append(headers, map[string]interface{}{"keyType": "other", "keyValue": key, "valueType": "other", "valueValue": str(m[key])})
			}
		}
		if headers == nil {
			headers = []interface{}{}
		}
		out["headers"] = headers
	case "http response":
		out["statusCode"] = str(num(c["statusCode"]))
	case "mqtt in":
		out["broker"] = str(c["broker"])
		out["topic"] = str(c["topic"])
		out["qos"] = str(num(c["qos"]))
		out["datatype"] = "auto-detect"
	case "mqtt out":
		out["broker"] = str(c["broker"])
		out["topic"] = str(c["topic"])
		out["qos"] = str(num(c["qos"]))
		out["retain"] = strconv.FormatBool(boolean(c["retain"]))
	case "mqtt-broker":
		host, port := splitHostPort(str(c["url"]))
		out["broker"], out["port"] = host, port
		out["clientid"] = str(c["clientId"])
		out["keepalive"] = str(num(c["keepAliveSec"]))
		out["cleansession"] = c["cleanSession"] != false
		out["usetls"] = boolean(c["useTLS"])
		out["protocolVersion"] = "4"
	case "tcp in":
		if c["server"] == false {
			out["server"] = "client"
		} else {
			out["server"] = "server"
		}
		out["host"] = str(c["host"])
		out["port"] = str(num(c["port"]))
		out["datatype"] = str(c["datatype"])
		if boolean(c["splitLines"]) {
			out["newline"] = "\\n"
		} else {
			out["newline"] = ""
		}
		out["datamode"] = "stream"
	case "tcp out":
		out["host"] = str(c["host"])
		out["port"] = str(num(c["port"]))
		out["beserver"] = "client"
	case "tcp request":
		out["server"] = str(c["host"])
		out["port"] = str(num(c["port"]))
		out["tout"] = str(num(c["timeoutMs"]))
		out["out"] = "time"
	case "udp in":
		out["port"] = str(num(c["port"]))
		out["datatype"] = str(c["datatype"])
	case "udp out":
		out["addr"] = str(c["host"])
		out["port"] = str(num(c["port"]))
	case "websocket in", "websocket out":
		out["server"] = str(c["server"])
	case "websocket-listener":
		out["path"] = str(c["path"])
	case "websocket-client":
		out["path"] = str(c["url"])
	case "tls-config":
		out["cert"] = str(c["cert"])
		out["key"] = str(c["key"])
		out["ca"] = str(c["ca"])
		out["servername"] = str(c["servername"])
		out["verifyservercert"] = c["verifyServerCert"] != false
	case "http proxy":
		out["url"] = str(c["url"])
		out["noproxy"] = toInterfaces(stringList(c["noProxy"]))
	default:
		for k, v := range c {
			if !structuralKeys[k] {
				out[k] = v
			}
		}
	}
	return out
}

var switchOperators = map[string]bool{
	"eq": true, "neq": true, "lt": true, "lte": true, "gt": true, "gte": true, "btwn": true, "cont": true, "regex": true,
	"true": true, "false": true, "null": true, "nnull": true, "empty": true, "nempty": true, "else": true,
}

func propertyType(pt string, w *warnings, node string) string {
	switch pt {
	case "", "msg":
		return "msg"
	case "flow", "global":
		return pt
	default:
		w.add("%s: property type %q is not supported; msg was used", node, pt)
		return "msg"
	}
}

func valueType(vt string, w *warnings) string {
	switch vt {
	case "", "str":
		return "str"
	case "num", "bool", "json", "env", "msg", "flow", "global":
		return vt
	default:
		w.add("value type %q is not supported; the value is used as a string", vt)
		return "str"
	}
}

func triggerPayload(opType, op string, w *warnings) map[string]interface{} {
	switch opType {
	case "pay", "payl":
		return map[string]interface{}{"type": "msg", "value": "payload"}
	case "nul":
		return map[string]interface{}{}
	case "date":
		w.add("trigger: a timestamp payload is not supported; a string was used")
		return typed("str", op)
	case "":
		return typed("str", op)
	default:
		return typed(valueType(opType, w), op)
	}
}

func exportTriggerPayload(v interface{}) (string, string) {
	vt, value := typedOf(v)
	switch {
	case vt == "":
		return "nul", ""
	case vt == "msg" && value == "payload":
		return "pay", ""
	default:
		return vt, value
	}
}

func fileNameType(t string) string {
	switch t {
	case "msg", "flow", "global", "env", "str":
		return t
	default:
		return "str"
	}
}

func bufferType(t string) string {
	if t == "utf8" {
		return "utf8"
	}
	return "buffer"
}

func unit(name string, table map[string]float64, fallback float64) float64 {
	if factor, ok := table[name]; ok {
		return factor
	}
	return fallback
}

func firstPositive(value, fallback float64) float64 {
	if value > 0 {
		return value
	}
	return fallback
}

func splitCSVList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func splitHostPort(url string) (string, string) {
	rest := url
	if i := strings.Index(rest, "://"); i >= 0 {
		rest = rest[i+3:]
	}
	if i := strings.LastIndex(rest, ":"); i >= 0 {
		return rest[:i], rest[i+1:]
	}
	return rest, "1883"
}

func sortedStringKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}
