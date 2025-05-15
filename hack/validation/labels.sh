#!/bin/bash
set -euo pipefail
# Labels Validation

# # Adding validation for nodepool

rule=$'self.all(x, x in
    [
        "karpenter.k8s.oracle/ocinodeclass",
        "karpenter.k8s.oracle/instance-shape-name",
        "karpenter.k8s.oracle/instance-cpu",
        "karpenter.k8s.oracle/instance-memory",
        "karpenter.k8s.oracle/instance-gpu",
        "karpenter.k8s.oracle/instance-network-bandwidth",
        "karpenter.k8s.oracle/instance-max-vnics",
        "karpenter.k8s.oracle/is-flexible"
    ]
    || !x.find("^([^/]+)").endsWith("karpenter.k8s.oracle")
)
'
# above regex: everything before the first '/' (any characters except '/' at the beginning of the string)

rule=${rule//\"/\\\"}            # escape double quotes
rule=${rule//$'\n'/}             # remove newlines
rule=$(echo "$rule" | tr -s ' ') # remove extra spaces

printf -v expr '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.template.properties.metadata.properties.labels.x-kubernetes-validations +=
    [{"message": "label domain \\"karpenter.k8s.oracle\\" is restricted", "rule": "%s"}]' "$rule"
yq eval "${expr}" -i pkg/apis/crds/karpenter.sh_nodepools.yaml

printf -v expr '.spec.versions[1].schema.openAPIV3Schema.properties.spec.properties.template.properties.metadata.properties.labels.x-kubernetes-validations +=
    [{"message": "label domain \\"karpenter.k8s.oracle\\" is restricted", "rule": "%s"}]' "$rule"
yq eval "${expr}" -i pkg/apis/crds/karpenter.sh_nodepools.yaml