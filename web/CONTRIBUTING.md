# Contributing to Go-RED Web UI

## Development Setup

1. Clone the repository and navigate to web directory
2. Install dependencies: npm install
3. Start development server: npm run dev
4. Start Go-RED backend in another terminal
5. Open http://localhost:3000 in your browser

## Project Structure

- src/components/: React components
- src/hooks/: Custom React hooks
- src/types/: TypeScript type definitions
- src/utils/: Utility functions
- src/styles/: CSS styles
- src/test/: Test files

## Code Style

- Use TypeScript for type safety
- Follow React best practices
- Use Tailwind CSS for styling
- Keep components small and focused

## Testing

- npm test: Run all tests
- npm run test:coverage: Run tests with coverage
- npm run typecheck: Check TypeScript types
- npm run lint: Run ESLint

## Adding New Features

1. New Node Types: Add metadata in backend, UI auto-displays them
2. New Components: Create in src/components/, export from index.ts
3. New Hooks: Create in src/hooks/, export from index.ts
4. New Types: Add to appropriate file in src/types/, export from index.ts

## Pull Requests

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Open a Pull Request

## License

MIT License
