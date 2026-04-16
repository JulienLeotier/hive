# Federation

Link independent Hive deployments so they can share capabilities across
organisational boundaries without leaking task data.

## Connect

```bash
hive federation connect my-peer https://hive.partner.com \
  --shared code-review,translate \
  --ca ca.pem \
  --cert client.pem \
  --key client.key
```

`connect` is an alias for `add`. mTLS is optional — omit `--ca/--cert/--key`
for plaintext HTTPS (not recommended outside of dev).

## List / remove

```bash
hive federation list
hive federation remove my-peer
```

Stored in `federation_links` so links survive restarts.

## Capability sharing

Declare the capabilities you're willing to expose when registering a link via
`--shared`. The partner sees only those capability names (no agent internals,
no task history).

## Cross-hive routing

`task.Router.WithFederation(resolver)` lets the router fall back to a peer
hive when no local agent has the requested capability. When that happens the
router emits:

- `decision.task_routed` with `chosen=federation:<peer>`
- `task.federated` with `{task_type, hive_name, hive_url}`

Your outbound proxy can subscribe to `task.federated` and forward the task
to the remote hive over the mTLS link.

## TLS material

`federation.Store.TLSConfigFor(name)` builds a `*tls.Config` from the stored
PEMs; `BuildClient(name)` returns an `*http.Client` wired for mTLS so the
proxy doesn't have to re-parse the cert material per call.
