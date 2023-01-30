#!/bin/bash

baseurl=http://gateway.prometheus.sums.top
for uri in $(curl -sS $baseurl/api/v1/metrics | jq -r '
  .data[].push_time_seconds.metrics[0] |
  select((now - (.value | tonumber)) > 20) |
  (.labels as $labels | ["job", "instance"] | map(.+"/"+$labels[.]) | join("/"))
'); do
  echo $uri
  curl -XDELETE $baseurl/metrics/$uri | exit
  echo curl -XDELETE $baseurl/metrics/$uri
done