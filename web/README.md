# Go-RED Web UI

A React-based web interface for Go-RED, a Node-RED clone built in Go.

## Features

- **Flow Editor**: Visual flow editing with drag-and-drop nodes
- **Real-time Updates**: WebSocket-based communication with the Go-RED backend
- **Node Palette**: Categorized node types for easy access
- **Node Configuration**: Modal-based configuration for each node
- **Flow Management**: Create, edit, deploy, and undeploy flows
- **Responsive Design**: Works on desktop and cloud environments

## Tech Stack

- **Frontend**: React 18 + TypeScript
- **Styling**: Tailwind CSS
- **Flow Visualization**: React Flow
- **State Management**: Zustand (via custom hooks)
- **Build Tool**: Vite
- **Testing**: Vitest + Testing Library
- **Backend Communication**: REST API + WebSocket

## Project Structure

web/
- src/
  - components/ (React components)
  - hooks/ (Custom React hooks)
  - types/ (TypeScript type definitions)
  - utils/ (Utility functions)
  - styles/ (CSS styles)
  - test/ (Test files)
- package.json
- tsconfig.json
- vite.config.ts
- index.html

## Getting Started

### Prerequisites

- Node.js 18+
- npm or yarn
- Go-RED backend running on port 8080

### Installation

cd web
npm install

### Development

npm run dev

### Production Build

npm run build

### Running Tests

npm test
npm run test:coverage

## Configuration

Create a .env file:

VITE_API_BASE_URL=/api
VITE_PORT=3000

## Scripts

- npm run dev: Start development server
- npm run build: Build for production
- npm run lint: Run ESLint
- npm run typecheck: Check TypeScript types
- npm test: Run tests
- npm run test:coverage: Run tests with coverage

## License

MIT
