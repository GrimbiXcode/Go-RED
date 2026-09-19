import type { CSSProperties } from 'react';
import { Handle, Position } from '@xyflow/react';
import type { Port } from '../types/node';

/**
 * Shared port rendering for canvas nodes: small square connectors sitting
 * exactly on the node's left/right edge, evenly spaced when a node has
 * multiple ports on one side. Falls back to a single default input/output
 * pair only when a node type defines no ports at all.
 */

export interface NodeHandlesProps {
  inputPorts: Port[];
  outputPorts: Port[];
}

const PORT_SIZE = 10;

const defaultPort = (id: string): Port => ({ id, name: '', description: '', required: false });

function portStyle(index: number, total: number): CSSProperties {
  const fraction = (index + 1) / (total + 1);
  return {
    top: `calc(var(--gr-node-height) * ${fraction})`,
    transform: 'translateY(-50%)',
    width: PORT_SIZE,
    height: PORT_SIZE,
    borderRadius: 2,
    background: '#475569',
    border: '1px solid white',
  };
}

export function NodeHandles({ inputPorts, outputPorts }: NodeHandlesProps) {
  const noPortsDefined = inputPorts.length === 0 && outputPorts.length === 0;
  const inputs = noPortsDefined ? [defaultPort('input')] : inputPorts;
  const outputs = noPortsDefined ? [defaultPort('output')] : outputPorts;

  return (
    <>
      {inputs.map((port, i) => (
        <Handle
          key={port.id}
          type="target"
          position={Position.Left}
          id={port.id}
          title={port.name || undefined}
          style={portStyle(i, inputs.length)}
        />
      ))}
      {outputs.map((port, i) => (
        <Handle
          key={port.id}
          type="source"
          position={Position.Right}
          id={port.id}
          title={port.name || undefined}
          style={portStyle(i, outputs.length)}
        />
      ))}
      {outputs.length > 1 &&
        outputs.map((port, i) => (
          <div
            key={`label-${port.id}`}
            className="absolute left-full ml-2 text-[9px] leading-none text-gray-500 whitespace-nowrap pointer-events-none max-w-[8rem] truncate"
            style={{ top: `calc(var(--gr-node-height) * ${(i + 1) / (outputs.length + 1)})`, transform: 'translateY(-50%)' }}
            data-testid="port-label"
          >
            {port.name}
          </div>
        ))}
    </>
  );
}
