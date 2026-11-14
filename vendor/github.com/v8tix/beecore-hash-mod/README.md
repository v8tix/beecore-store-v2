# beecore-hash-mod

Argon2id password hashing for Go — a small shared module consumed by other V8TIX microservices
(`beecore-auth-v3-mod`, `beecore-stores`, `beecore-baskets`, etc.), not a microservice itself.

## Installation

```
go get github.com/v8tix/beecore-hash-mod
```

## Usage

```go
hash, err := hash.Argon2idHash("the-users-password")
if err != nil {
    // handle error
}

err = hash.CheckHash(hash, "the-users-password")
if err != nil {
    // invalid password, or a malformed/empty hash
}
```

## Branching strategy

* `master` always holds released, working code.
* `develop` is the integration branch where new work lands first.
* Both branches are protected: nobody can push to them directly, force-push them or delete them. Every change
  arrives through a pull request, which must pass the CI checks with all conversations resolved.
* Start a feature or fix from `develop` (`git switch -c feature/<name> develop`), open a pull request into
  `develop`, and merge it once CI is green.
* To release, open a pull request from `develop` into `master`. Use **Create a merge commit** (not squash or
  rebase) so the two branches keep a common history. After it is merged, tag the release on `master`, for
  example `git tag -a v1.0.4 -m "v1.0.4" && git push origin v1.0.4`.
