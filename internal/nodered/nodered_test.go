package nodered

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sample = `[
 {"id":"tab1","type":"tab","label":"Main","disabled":false,"info":"Demo"},
 {"id":"tab2","type":"tab","label":"Second","disabled":false,"info":""},
 {"id":"broker1","type":"mqtt-broker","name":"Local","broker":"localhost","port":"1883","clientid":"","usetls":false,"keepalive":"60","cleansession":true},
 {"id":"n1","type":"inject","z":"tab1","name":"Tick","repeat":"1","crontab":"","once":true,"topic":"t","payload":"tick","payloadType":"str","x":110,"y":100,"wires":[["n2"]]},
 {"id":"n2","type":"function","z":"tab1","name":"Count","func":"msg.count=1;\nreturn msg;","outputs":1,"x":300,"y":100,"wires":[["n3"]]},
 {"id":"n3","type":"switch","z":"tab1","name":"Route","property":"payload","propertyType":"msg","rules":[{"t":"eq","v":"tick","vt":"str"},{"t":"btwn","v":"1","vt":"num","v2":"5","v2t":"num"},{"t":"hask","v":"x","vt":"str"},{"t":"else"}],"checkall":"false","outputs":4,"x":480,"y":100,"wires":[["n4"],["n5"],[],["n5"]]},
 {"id":"n4","type":"debug","z":"tab1","name":"Out","active":true,"tosidebar":true,"console":false,"complete":"true","x":660,"y":60,"wires":[]},
 {"id":"n5","type":"change","z":"tab1","name":"","rules":[{"t":"set","p":"topic","pt":"msg","to":"changed","tot":"str"},{"t":"move","p":"payload","pt":"msg","to":"data","tot":"msg"}],"x":660,"y":140,"wires":[["n6"]]},
 {"id":"n6","type":"mqtt out","z":"tab1","name":"","topic":"out","qos":"1","retain":"true","broker":"broker1","x":860,"y":140,"wires":[]},
 {"id":"n7","type":"ui_chart","z":"tab1","name":"Chart","group":"g1","x":860,"y":200,"wires":[[]]},
 {"id":"n8","type":"delay","z":"tab2","name":"","pauseType":"rate","timeout":"5","timeoutUnits":"seconds","rate":"2","nbRateUnits":"1","rateUnits":"minute","drop":true,"x":200,"y":100,"wires":[[]]},
 {"id":"n9","type":"comment","z":"tab2","name":"Note","info":"hello","x":400,"y":100,"wires":[]}
]`

func TestImport_MapsTabsNodesWiresAndConfig(t *testing.T) {
	result, err := Import([]byte(sample))
	require.NoError(t, err)
	require.Len(t, result.Flows, 2)

	main := result.Flows[0]
	assert.Equal(t, "Main", main.Name)
	assert.Equal(t, "Demo", main.Description)
	assert.Len(t, main.Nodes, 8, "seven tab nodes plus the referenced broker")

	inject := main.Nodes["n1"]
	assert.Equal(t, "inject", inject.Type)
	assert.Equal(t, "Tick", inject.Name)
	assert.Equal(t, map[string]interface{}{"payload": "tick"}, inject.Config["payload"])
	assert.Equal(t, "t", inject.Config["topic"])
	assert.Equal(t, float64(1000), inject.Config["interval"])
	assert.Equal(t, true, inject.Config["injectOnce"])
	assert.Equal(t, float64(40), inject.X)
	assert.Equal(t, float64(85), inject.Y)

	fn := main.Nodes["n2"]
	assert.Equal(t, "msg.count=1;\nreturn msg;", fn.Config["code"])
	assert.Equal(t, true, fn.Config["useMsg"])

	sw := main.Nodes["n3"]
	assert.Equal(t, map[string]interface{}{"type": "msg", "path": "payload"}, sw.Config["property"])
	assert.Equal(t, false, sw.Config["checkAll"])
	rules := sw.Config["rules"].([]interface{})
	require.Len(t, rules, 4, "unsupported rules stay so the ports keep lining up")
	assert.Equal(t, map[string]interface{}{"operator": "eq", "value": map[string]interface{}{"type": "str", "value": "tick"}}, rules[0])
	assert.Equal(t, map[string]interface{}{"type": "num", "value": "5"}, rules[1].(map[string]interface{})["value2"])
	assert.Equal(t, "else", rules[3].(map[string]interface{})["operator"])

	assert.Equal(t, "full", main.Nodes["n4"].Config["output"])

	change := main.Nodes["n5"].Config["rules"].([]interface{})
	require.Len(t, change, 2)
	assert.Equal(t, map[string]interface{}{"action": "set", "target": map[string]interface{}{"type": "msg", "path": "topic"}, "value": map[string]interface{}{"type": "str", "value": "changed"}}, change[0])
	assert.Equal(t, map[string]interface{}{"type": "msg", "path": "data"}, change[1].(map[string]interface{})["moveTo"])

	out := main.Nodes["n6"]
	assert.Equal(t, "broker1", out.Config["broker"])
	assert.Equal(t, float64(1), out.Config["qos"])
	assert.Equal(t, true, out.Config["retain"])

	broker := main.Nodes["broker1"]
	require.NotNil(t, broker, "the referenced config node is copied into the flow")
	assert.Equal(t, "tcp://localhost:1883", broker.Config["url"])
	assert.Equal(t, float64(60), broker.Config["keepAliveSec"])

	chart := main.Nodes["n7"]
	assert.Equal(t, "ui_chart", chart.Type)
	assert.Equal(t, "g1", chart.Config["group"], "unknown types keep their raw properties")

	// Wires: single-output nodes use "output", the switch uses its rule index.
	var pairs []string
	for _, c := range main.Connections {
		pairs = append(pairs, c.SourceNode+":"+c.SourcePort+"->"+c.TargetNode+":"+c.TargetPort)
	}
	assert.ElementsMatch(t, []string{"n1:output->n2:input", "n2:output->n3:input", "n3:0->n4:input", "n3:1->n5:input", "n3:3->n5:input", "n5:output->n6:input"}, pairs)

	second := result.Flows[1]
	assert.Equal(t, "Second", second.Name)
	delay := second.Nodes["n8"]
	assert.Equal(t, "rate", delay.Config["mode"])
	assert.Equal(t, float64(2), delay.Config["rateLimit"])
	assert.Equal(t, float64(60000), delay.Config["rateIntervalMs"])
	assert.Equal(t, float64(5000), delay.Config["delayMs"])
	assert.Equal(t, true, delay.Config["dropIntermediate"])
	assert.Equal(t, "hello", second.Nodes["n9"].Config["text"])
	assert.Equal(t, "Note", second.Nodes["n9"].Name)

	joined := strings.Join(result.Warnings, "\n")
	assert.Contains(t, joined, `"ui_chart"`)
	assert.Contains(t, joined, `"hask"`)
}

func TestImport_RejectsNonArrays(t *testing.T) {
	_, err := Import([]byte(`{"name":"x"}`))
	assert.Error(t, err)
	assert.True(t, IsExport([]byte("  [ {}]")))
	assert.False(t, IsExport([]byte(`{"nodes":{}}`)))
}

func TestImport_NodesWithoutTabAndSubflows(t *testing.T) {
	data := `[
	 {"id":"sf1","type":"subflow","name":"My subflow","in":[],"out":[]},
	 {"id":"i1","type":"inject","z":"sf1","payload":"","payloadType":"date","x":1,"y":1,"wires":[[]]},
	 {"id":"i2","type":"inject","z":"orphan","payload":"","payloadType":"date","x":1,"y":1,"wires":[["i3"]]},
	 {"id":"i3","type":"debug","z":"orphan","complete":"false","x":1,"y":1,"wires":[]}
	]`
	result, err := Import([]byte(data))
	require.NoError(t, err)
	require.Len(t, result.Flows, 1, "nodes whose tab is missing land in one flow; subflow members are skipped")
	assert.Equal(t, "Imported flow", result.Flows[0].Name)
	assert.Len(t, result.Flows[0].Nodes, 2)
	assert.Len(t, result.Flows[0].Connections, 1)
	joined := strings.Join(result.Warnings, "\n")
	assert.Contains(t, joined, "subflow")
	assert.Contains(t, joined, "timestamp")
}

func TestExport_RoundTrip(t *testing.T) {
	result, err := Import([]byte(sample))
	require.NoError(t, err)
	main := result.Flows[0]

	exported := Export(main)
	require.Equal(t, "tab", exported[0]["type"])
	assert.Equal(t, "Main", exported[0]["label"])

	byID := map[string]Raw{}
	for _, raw := range exported[1:] {
		byID[str(raw["id"])] = raw
	}
	assert.Equal(t, [][]string{{"n4"}, {"n5"}, {}, {"n5"}}, byID["n3"]["wires"])
	assert.Equal(t, 4, byID["n3"]["outputs"])
	assert.Equal(t, "tab1", byID["n1"]["z"])
	assert.Equal(t, float64(110), byID["n1"]["x"])
	_, brokerHasZ := byID["broker1"]["z"]
	assert.False(t, brokerHasZ, "config nodes have no tab")
	assert.Equal(t, "localhost", byID["broker1"]["broker"])
	assert.Equal(t, "1883", byID["broker1"]["port"])
	assert.Equal(t, "true", byID["n4"]["complete"])
	assert.Equal(t, "1", byID["n1"]["repeat"])

	// What we export, we can import again without losing nodes or wires.
	data, err := json.Marshal(exported)
	require.NoError(t, err)
	again, err := Import(data)
	require.NoError(t, err)
	require.Len(t, again.Flows, 1)
	assert.Len(t, again.Flows[0].Nodes, len(main.Nodes))
	assert.Len(t, again.Flows[0].Connections, len(main.Connections))
	assert.Equal(t, main.Nodes["n5"].Config, again.Flows[0].Nodes["n5"].Config)
	assert.Equal(t, main.Nodes["n3"].Config, again.Flows[0].Nodes["n3"].Config)
	assert.Equal(t, main.Nodes["n1"].Config, again.Flows[0].Nodes["n1"].Config)
}
