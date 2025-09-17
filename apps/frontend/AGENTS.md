# AGENTS: Frontend (Next.js) Guidance

## App Summary
- Next.js 14 app with TypeScript, Tailwind, shadcn/ui
- Web3: Wagmi + viem, WalletConnect/RainbowKit (planned)
- Talks to smart contracts (ABIs from `packages/contracts/out`)
- Talks to node agent API at `http://localhost:8080` in dev

## Coding Rules
- TypeScript strict; no `any` on public APIs
- Prefer server components; add client directive only when needed
- Add loading and error states for async UI
- Keep components small; colocate hooks in `apps/frontend/lib` or stores in `apps/frontend/store`
- Do not hardcode secrets; only `NEXT_PUBLIC_*` in the browser

## Web3 Conventions
- Use `viem` clients and `wagmi` hooks
- Import contract ABIs from `packages/contracts/out/*/*.json`
- Read calls: prefer public view functions
- Write calls: surface transaction status and errors to the user
- Chain config via `NEXT_PUBLIC_CHAIN_ID`

## UI/UX Conventions
- Tailwind classes for layout; shadcn/ui for primitives
- Accessible defaults: labels, aria, focus states
- Empty, loading, and error states for lists and async content

## Testing
- Prefer lightweight component tests for critical flows
- Mock chain calls and node agent API

## Commands
- Dev: `pnpm --filter @apps/frontend dev`
- Build: `pnpm --filter @apps/frontend build`

## Guardrails
- Do not invent ABIs; import from contracts `out/`
- Validate and sanitize any user input before passing to contracts or API
- Avoid client-side private keys or sensitive data

## Common Tasks
- Add page/component with proper states and types
- Wire up contract read/write via `wagmi`
- Call node agent and render results

