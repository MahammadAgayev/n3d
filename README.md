## Note
The repo still in initial development and may not work on occasionally

## N3D
This is the tool for local nomad development

Currently this is in early development and only supports creating one test cluster.

Tool may simple create cluster and destroy it. The hashistack UIs will be available on default ports after cluster provisioning.
The commands to play with it are below. 

```
n3d cluster create my-test-cluster
n3d cluster delete my-test-cluster
```

### Next features
- Persistence needs to be implemented. 
- Support for multiple clusters (Load balancing for UIs and tools)

