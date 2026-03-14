# Schemas for Kubernetes Custom Resource Definitions (CRDs)

<!-- cSpell: words datree -->

This directory contains JSON Schema files for Kubernetes Custom Resource
Definitions (CRDs) used in our platform. These schemas are used for validating
the structure of CRD instances and ensuring they conform to expected formats.

The schemas are organized by API group and resource kind, following the naming
convention:

```
<Group>/<ResourceKind>_<ResourceAPIVersion>.json
```

They are placed here to override the datree schema catalog for any custom
resources that are not available there, or to provide more up-to-date schemas
for our specific use cases. The `validate-helmfile.sh` script references these
schemas when validating Helm charts against Kubernetes API specifications.
