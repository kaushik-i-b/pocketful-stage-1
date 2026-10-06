# Pocketful stage 1

Team: Kaushik Itagi B (`kaushik-i-b`). Public repository: https://github.com/kaushik-i-b/pocketful-stage-1.

This repository is the Pocketful stage 1 result from one Band Desktop room. The service is in `stage-1/`. The factory that produced it is described in `FACTORY.md`. Seat instructions are in `mandates/`. The room log belongs in `room.json` and is not in this tree yet. The official full-session download has not been saved.

## What this tree claims

Shipped stage 1 checks passed for commit `7e0beb83b35c3091bf054c7607c5f1c7b7613a7a`. That is the stage this entry claims. The same isolated run continued into the stage 2 suite and failed there. This repository does not claim stage 2.

The shipped checks are a portion of the hidden suite used for judging. A pass here is not a claim that the hidden suite passed.

## How to run the service

From `stage-1/`, follow `RUN.md`:

```sh
docker build -t pocketful-stage1 . && docker run --rm -p 8080:8080 -e PORT=8080 pocketful-stage1
```

`GET /health` returns `{"status":"ok"}`.

## How to read the history

The service commits are kept as the seats made them:

- `5e7d08797aec4b15cc0027326ddb526cc4fff662` — empty result repository
- `9efc373661c0135a72f75e53d513b4d007be9040` — Developer, stage 1 service
- `7e0beb83b35c3091bf054c7607c5f1c7b7613a7a` — Developer, exact minor units and unknown handles

`main` was fast-forwarded to the second Developer commit. Those commits were not squashed or rewritten.
