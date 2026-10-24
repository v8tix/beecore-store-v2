# beecore-store-v2

## Branching strategy

* `main` always holds released, working code.
* `develop` is the integration branch where new work lands first.
* Both branches are protected: nobody can push to them directly, force-push them or delete them. Every change
  arrives through a pull request, which must pass CI (when configured) with all conversations resolved.
* Start a feature or fix from `develop` (`git switch -c feature/<name> develop`), open a pull request into
  `develop`, and merge it once it's ready.
* To release, open a pull request from `develop` into `main`. Use **Create a merge commit** (not squash or
  rebase) so the two branches keep a common history.
