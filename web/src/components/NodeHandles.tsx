import { useEffect, useRef, type CSSProperties } from 'react';
import { Handle, Position, useNodeId, useUpdateNodeInternals } from '@xyflow/react';
import type { Port } from '../types/node';

/**
 * Port rendering for canvas nodes: round connectors on the node's edge,
 * evenly spaced, drawn in the node's own color (`--node-color`, set by
 * NodeShell). Multi-output nodes get a small label next to each output.
 * Falls back to one input and one output when a type declares no ports.
 *
 * xyflow measures handle positions only when a node's size changes, and a
 * node keeps its 36 px height whatever its ports, so a change of ports (a
 * switch rule added, the node types arriving after the flow) has the node
 * measured again; otherwise wires would start at the old positions or, on a
 * new port, not be drawn or connectable at all.
 */

export interface NodeHandlesProps {
  inputPorts: Port[];
  outputPorts: Port[];
}

const PORT_SIZE = 10;

const defaultPort = (id: string): Port => ({ id, name: '', description: '', required: false });

/** Identifies a node's handle layout: which ports there are, in which order. */
export function portLayoutKey(inputs: Port[], outputs: Port[]): string {
  return JSON.stringify([inputs.map((port) => port.id), outputs.map((port) => port.id)]);
}

function portTop(index: number, total: number): string {
  return `calc(var(--gr-node-height) * ${(index + 1) / (total + 1)})`;
}

function portStyle(index: number, total: number): CSSProperties {
  return {
    top: portTop(index, total),
    transform: 'translateY(-50%)',
    width: PORT_SIZE,
    height: PORT_SIZE,
    minWidth: PORT_SIZE,
    minHeight: PORT_SIZE,
    borderRadius: '50%',
    background: 'var(--bg-panel)',
    border: '2px solid var(--node-color)',
  };
}

export function NodeHandles({ inputPorts, outputPorts }: NodeHandlesProps) {
  const noPortsDefined = inputPorts.length === 0 && outputPorts.length === 0;
  const inputs = noPortsDefined ? [defaultPort('input')] : inputPorts;
  const outputs = noPortsDefined ? [defaultPort('output')] : outputPorts;

  const nodeId = useNodeId();
  const updateNodeInternals = useUpdateNodeInternals();
  const layout = portLayoutKey(inputs, outputs);
  const measuredLayout = useRef(layout);
  useEffect(() => {
    if (measuredLayout.current === layout) return;
    measuredLayout.current = layout;
    if (nodeId) updateNodeInternals(nodeId);
  }, [layout, nodeId, updateNodeInternals]);

  return (
    <>
      {inputs.map((port, i) => (
        <Handle key={port.id} type="target" position={Position.Left} id={port.id} title={port.name || undefined} style={portStyle(i, inputs.length)} />
      ))}
      {outputs.map((port, i) => (
        <Handle key={port.id} type="source" position={Position.Right} id={port.id} title={port.name || undefined} style={portStyle(i, outputs.length)} />
      ))}
      {outputs.length > 1 &&
        outputs.map((port, i) => (
          <div
            key={`label-${port.id}`}
            className="absolute left-full ml-2.5 text-2xs leading-none text-muted whitespace-nowrap pointer-events-none max-w-[8rem] truncate"
            style={{ top: portTop(i, outputs.length), transform: 'translateY(-50%)' }}
            data-testid="port-label"
          >
            {port.name}
          </div>
        ))}
    </>
  );
}
