# AGENTS: Smart Contracts (Solidity) Guidance

## Package Summary
- Solidity 0.8.x with Foundry
- Uses OpenZeppelin libraries
- Contracts: `NodeRegistry`, `JobEscrow`

## Coding Rules
- Follow checks-effects-interactions
- Emit events for state changes
- Use custom errors, not `require` strings
- Avoid copying nested calldata arrays directly; manually push to storage when needed

## Security
- Validate all inputs; reject zero addresses and invalid amounts
- Use `block.timestamp` carefully; check deadlines
- Protect against reentrancy; keep external calls last

## Testing Policy
- Unit tests for success and failure cases
- Signature verification tests (EIP-712)
- Gas snapshot if time permits

## Commands
- Build: `forge build`
- Test: `forge test -vvv`
- Deploy scripts in `packages/contracts/script/`

## Conventions
- Use OpenZeppelin implementations where possible
- Keep contracts minimal; move complex logic off-chain
- Use events for off-chain indexing

## Common Tasks
- Add or update contract with events and errors
- Write Foundry tests for new logic
- Update ABI artifacts in `out/` and import from frontend

