klog v1 shim
============

This is a fork of [`k8s.io/klog`](https://github.com/kubernetes/klog) v1 that proxies everything to klog v2; so that
klog v2 can coexist with libraries still using klog v1.

Using
-----

```shell
go mod edit -replace=k8s.io/klog=github.com/datawire/klog
```

As simple as that, code that uses klog v1 will be calling in to klog v2.

Changes from klog v1.0.0
------------------------

Backported fixes:
- Backport fix for running on Windows Nano (https://github.com/kubernetes/klog/issues/124)
- Backport fix for setting `-log_backtrace_at` to an empty string (https://github.com/kubernetes/klog/pull/115)
- Backport fix for `-add_dir_header` not working correctly at all (https://github.com/kubernetes/klog/pull/101)

Changes:
- Backport change from v2 where whenever we log a fatal message to stderr in addition to the normal log, also write to
  stderr the stacktrace that is written to the normal log (https://github.com/kubernetes/klog/pull/79)
- The `klogv1.Stats` variable is now of type `*OutputStats` instead of `OutputStats`
- Setting the global `klogv1.MaxSize` variable no longer has any affect, you must set `klogv2.MaxSize` to have an
  affect.

Limitations compared to klog v2
-------------------------------

- Setting `klogv1.MaxSize` has no affect; you must set `klogv2.MaxSize` to have an affect.
- When using `klogv2.SetLogger(logr.Logger)`, calls to `klogv1.V(verbosity)` do not inform the underlying `logr.Logger`
  is of what the verbosity is; the logger's `.V(verbosity)` method is not called.  The verbosity value is used by klog
  to decide whether to call in to the logger at all, but is not passed to to the logger.  In order for the logger to be
  informed of the verbosity, you must use `klogv2.V(verbosity)`.
