import js from '@eslint/js';
import globals from 'globals';
import reactHooks from 'eslint-plugin-react-hooks';
import reactRefresh from 'eslint-plugin-react-refresh';
import tseslint from 'typescript-eslint';

export default tseslint.config(
  { ignores: ['dist', 'node_modules', 'coverage', 'src/types/generated.ts'] },
  {
    files: ['**/*.{ts,tsx}'],
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    languageOptions: {
      ecmaVersion: 2022,
      globals: { ...globals.browser, ...globals.node },
    },
    plugins: {
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      // Fast-refresh boundaries are a dev-only concern; revisit with the
      // Phase 1 store refactor.
      'react-refresh/only-export-components': 'off',
      // Production code must not log; warn/error stay allowed for real
      // failure reporting.
      'no-console': ['error', { allow: ['warn', 'error'] }],
      // Config maps and wire payloads are untyped today (see Schema v2 in
      // docs/NEXT_LEVEL_PLAN.md); tighten once they are.
      '@typescript-eslint/no-explicit-any': 'off',
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_', caughtErrors: 'none' },
      ],
    },
  }
);
