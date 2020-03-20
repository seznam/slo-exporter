# Normalzier

|                |              |
|----------------|--------------|
| `moduleName`   | `normalizer` |
| Module type    | `processor`  |
| Input event    | `raw`        |
| Output event   | `raw`        |

This module allows you to normalize some event data. 
By normalization it is meant to remove unique sequences from the data so they can be grouped
and then umber of unique records is not that high.

It has some built-it normalization for some common patterns such as image names,
hashes, ips, only number sequences between slashes etc. Also it supports custom regexp replacement rules.

It is useful for example to remove ids form REST endpoint path `/user/123/info` to `user/:id/info`

`moduleConfig`
```yaml
getParamWithEventIdentifier: "operationName"
# List of replace rules to be applied on the HTTP request path
replaceRules:
  # Regular expression to match the path
  - regexp: "/api/v1/ppchit/rule/[0-9a-fA-F]{5,16}"
    # Replacement of the matched path
    replacement: "/api/v1/ppchit/rule/0"
# If hashes in the path should be replaced (MD5, SHA1, ...).
sanitizeHashes: true
# If number sequences in the path should be replaced eg `/foo/123/bar`.
sanitizeNumbers: true
# If UIDs in the path should be replaced.
sanitizeUids: true
# If IPs in the path should be replaced (v4, v6).
sanitizeIps:     true
# If image names should be masked (.png, .jpg, .gif ...).
sanitizeImages:  true
# If font file names in the path should be replaced (.ttf, .woff, ...)
sanitizeFonts:   true
```

