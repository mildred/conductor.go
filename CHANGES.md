Changelog
=========

v0.0.82
-------

### Changes

- The setting `auto_restart` has been deprecated in favor of `static_deployments`
  which means the opposite. Whereas the default value for `auto_restart` is
  `true`, the default value for `static_deplotments` is `false`.

v0.0.81 or before
-----------------

### Breaking changes

- Policies are now directories that must contain a single file `policy.json`.
  You must manually migrate all your policies.
