# Market-Based Allocation

Instead of always picking the first capable agent, let agents bid on tasks and
pick the winner by price / speed / reputation.

## Workflow opt-in

Set the allocation strategy at the workflow level:

```yaml
name: my-workflow
allocation: market          # default is "capability-match"
tasks:
  - name: review
    type: code-review
```

Valid values: `capability-match` (default), `market`, `round-robin`. The parser
rejects anything else.

## CLI

```bash
hive auction open <task-id> --strategy lowest-cost    # or fastest, best-reputation
# …agents submit bids…
hive auction close <auction-id>
hive wallet balance reviewer
hive wallet credit  reviewer 100
```

- `open` creates a row in `auctions`, `SubmitBid` appends to `bids`, `close`
  picks a winner via `market.NewAuction(nil).SelectWinner(bids, strategy)`
  and emits `task.auction.won` with `{auction_id, bid_id, agent, price}`.
- `cancel` voids an auction without a winner.

## Strategies

`market.SelectWinner` supports:

- `lowest-cost` — min price
- `fastest` — min estimated duration
- `best-reputation` — max reputation score
- default — weighted blend `cost*0.4 + 1/(dur+1)*0.3 + rep*0.3`

## Token wallet

- `Credit` / `Debit` are atomic; debit refuses when balance would go negative.
- `Balance` returns 0 for agents without a wallet row (not an error).
- `hive agent stats <name>` surfaces bids/wins + token balance.

## Bid/win stats

```bash
hive agent stats reviewer
# Agent: reviewer (id=…)
#   Health:       healthy
#   Trust level:  autonomous
#   Total tasks:  42 (success=40, failed=2)
#   Error rate:   4.76%
#   Bids:         12 (won=8, rate=66.7%)
#   Token balance: 95.00
```
