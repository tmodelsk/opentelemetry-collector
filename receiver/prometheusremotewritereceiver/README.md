# Prometheus Remote Write Receiver

Supported pipeline types: metrics

## Getting Started

All that is required to enable the Prometheus Remote Write receiver is to include it in the
receiver definitions.

```yaml
receivers:
  prometheusremotewrite:
```

The following settings are configurable:

- `endpoint` (default = 0.0.0.0:19291): host:port to which the receiver is going
  to receive data.
  
