# HTTP/TCP Troubleshooting Tool

A Go-based tool for troubleshooting HTTP/TCP connections. It can act as a client, backend server, or both simultaneously.

## Features

- **Client Mode**: Makes HTTP requests to configured endpoints with detailed diagnostics
  - **Rate limiting**: Control N requests per second per endpoint
  - Connection diagnostics (DNS, TCP, TLS, TTFB)
  - Configurable retries
- **Backend Mode**: HTTP server with configurable responses
  - **Drop simulation**: Close connections without response (configurable %)
  - **Idle simulation**: Keep connections open without responding (configurable %)
  - Artificial delays
  - Custom status codes and headers
- **Both Mode**: Client and server running simultaneously
- **Prometheus Metrics**: `/metrics` endpoint with detailed client and backend metrics
- **Structured Logging**: Configurable log levels (debug, info, warn, error)
- **Graceful Shutdown**: Proper signal handling

## Installation

You can build the application locally or use the container image in OpenShift, available in the [Quay.io](https://quay.io/repository/mparrade/test-backend) registry.

### Build locally

```bash
go build -o ./bin/test-backend
```

#### Usage
Use default configuration (config/config.yaml)
```bash
./bin/test-backend
```

Specify configuration file
```bash
./bin/test-backend -config my-config.yaml
```

### Run in OpenShift
Using the container image from Quay.io
```bash
# Create the backend namespace
oc create namespace test-backend

# Apply the backend manifests
oc apply -k manifests/backend --namespace test-backend

# Create the client namespace
oc create namespace test-client

# Get the backend route host and update the client configuration
ROUTE_HOST=$(oc get route test-backend -n test-backend -o jsonpath='{.spec.host}')
sed -i "s/\${YOUR_ROUTE_HOST}/$ROUTE_HOST/" manifests/client/configmap-http-config-client.yaml

# Apply the client manifests
oc apply -k manifests/client --namespace test-client
```
then scale the client deployment to 10 replicas
```bash
oc scale deployment http-client --replicas=10 -n test-client
```


## Configuration

The `config/config.yaml` (Or configmaps in OpenShift) file controls the application behavior:

```yaml
# Type: client, backend, or both
type: both

# Client configuration
client:
  timeout: 0s           # Global execution duration (0s = run indefinitely)
  request_timeout: 30s  # Timeout for individual HTTP requests
  interval: 5s          # Default interval (if requests_per_second is not set)
  endpoints:
    - name: "Health Check"
      url: "http://localhost:8080/health"
      method: GET
      retries: 2
      requests_per_second: 2  # 2 requests per second
    
    - name: "API Test"
      url: "http://localhost:8080/api/test"
      method: POST
      headers:
        Content-Type: application/json
      body: '{"message": "test"}'
      retries: 1
      requests_per_second: 0.5  # 1 request every 2 seconds

# Backend configuration
backend:
  port: 8080
  endpoints:
    - path: /health
      method: GET
      status_code: 200
      body: "OK"
    
    - path: /unreliable
      method: GET
      status_code: 200
      drop_percent: 30      # Close 30% of connections without response
      idle_percent: 20      # Leave 20% of connections idle
      idle_duration: 10s    # Idle duration
      body: "Unreliable response"

# Logging configuration
logging:
  level: info     # debug, info, warn, error
  verbose: true   # Include detailed diagnostics
```

Testing in OpenShift, with the next configuration, we can estimate around 25k simultaneous connections, 
multiplying the 5s delay at the backend, by 500 requests per second at each client, and deploying 10 replicas of the client deployment.

Backend:
```yaml
type: backend
backend:
  port: 8080
  endpoints:
    - path: /unreliable
      method: GET
      status_code: 200
      drop_percent: 0      # Drop 30% of connections
      idle_percent: 100      # Idle 20% of connections
      idle_duration: 5s    # Keep idle connections for 10 seconds
      body: "Unreliable endpoint response"
```
Client:
```yaml
type: client
client:
  timeout: 120s           # Global execution duration (0s = run indefinitely)
  request_timeout: 30s  # Timeout for individual HTTP requests
  interval: 1s  # Default interval (used if requests_per_second is not set)
  endpoints:
    - name: "route-backend"
      url: "http://${YOUR_ROUTE_HOST}/unreliable"
      method: GET
      retries: 3
      requests_per_second: 500
```

## Operation Modes

### Client Mode

```yaml
type: client
client:
  # ... endpoint configuration
```

Makes periodic HTTP requests to configured endpoints and displays:
- Request and response status
- DNS, TCP, TLS timings
- Time to First Byte (TTFB)
- Headers and body (in verbose mode)

**Rate Limiting**: Configure `requests_per_second` per endpoint to control request rate:
- `requests_per_second: 2` → 2 requests per second
- `requests_per_second: 0.5` → 1 request every 2 seconds
- If not specified, uses the global `interval`

### Backend Mode

```yaml
type: backend
backend:
  # ... endpoint configuration
```

Starts an HTTP server that responds according to configuration:
- Custom status codes
- Custom headers
- Custom body
- Artificial delays to simulate latency
- **Drop connections**: Close connections without responding (configurable by %)
- **Idle connections**: Keep connections open without responding (configurable by % and duration)

### Both Mode

```yaml
type: both
client:
  # ... client configuration
backend:
  # ... backend configuration
```

Runs client and server simultaneously. Useful for:
- Loopback tests
- Simulating complete scenarios
- Troubleshooting local connections

## Usage Examples

### Troubleshooting an External Endpoint

```yaml
type: client
client:
  timeout: 10s
  interval: 2s
  endpoints:
    - name: "External API"
      url: "https://api.example.com/status"
      method: GET
      retries: 3
logging:
  level: debug
  verbose: true
```

### Simulating a Server with Latency and Failures

```yaml
type: backend
backend:
  port: 8080
  endpoints:
    - path: /fast
      method: GET
      status_code: 200
      body: "Fast response"
    
    - path: /slow
      method: GET
      status_code: 200
      delay: 5s
      body: "Slow response"
    
    - path: /unreliable
      method: GET
      status_code: 200
      drop_percent: 30      # Close 30% of connections
      idle_percent: 20      # Leave 20% idle for 15s
      idle_duration: 15s
      body: "Sometimes works"
    
    - path: /error
      method: GET
      status_code: 500
      body: '{"error": "Internal server error"}'
logging:
  level: info
  verbose: false
```

### Complete Test with Rate Limiting and Failure Simulation

```yaml
type: both
client:
  timeout: 5m           # Run for 5 minutes then stop
  request_timeout: 10s  # Fail requests taking longer than 10s
  interval: 3s
  endpoints:
    - name: "High Frequency Test"
      url: "http://localhost:8080/test"
      method: POST
      headers:
        Content-Type: application/json
      body: '{"test": true}'
      retries: 3
      requests_per_second: 5  # 5 requests per second
    
    - name: "Unreliable Service Test"
      url: "http://localhost:8080/unreliable"
      method: GET
      retries: 2
      requests_per_second: 1
backend:
  port: 8080
  endpoints:
    - path: /test
      method: POST
      status_code: 200
      headers:
        Content-Type: application/json
      body: '{"status": "ok"}'
    
    - path: /unreliable
      method: GET
      status_code: 200
      drop_percent: 25
      idle_percent: 15
      idle_duration: 10s
      body: '{"status": "maybe"}'
logging:
  level: info
  verbose: true
```

## Sample Output

```
[2025-12-02 12:27:07.667] [INFO] === HTTP/TCP Troubleshooting Tool ===
[2025-12-02 12:27:07.667] [INFO] Mode: both
[2025-12-02 12:27:07.667] [INFO] Prometheus metrics initialized
[2025-12-02 12:27:07.667] [INFO] Starting HTTP client...
[2025-12-02 12:27:07.667] [INFO] Registering Prometheus metrics endpoint: /metrics
[2025-12-02 12:27:07.667] [INFO] Starting HTTP backend server on port 8080...
[2025-12-02 12:27:07.667] [INFO] → [Local Health Check] GET http://localhost:8080/health (attempt 1)
[2025-12-02 12:27:07.668] [INFO] ← GET /health from [::1]:51360
[2025-12-02 12:27:07.668] [INFO] → GET /health -> 200 (took 14.397µs)
[2025-12-02 12:27:07.669] [INFO] ← [Local Health Check] Status: 200, Size: 2 bytes, Duration: 232.408µs
```

## Prometheus Metrics

The application exposes detailed metrics at the `/metrics` endpoint when running in `backend` or `both` mode.

### Accessing Metrics

```bash
# View all metrics
curl http://localhost:8080/metrics

# Filter client metrics
curl http://localhost:8080/metrics | grep http_client

# Filter backend metrics
curl http://localhost:8080/metrics | grep http_backend
```

### Client Metrics

- **http_client_requests_total**: Total HTTP requests (labels: endpoint, method, status_code)
- **http_client_request_duration_seconds**: Request duration (histogram)
- **http_client_request_errors_total**: Total errors (labels: endpoint, method, error_type)
- **http_client_dns_duration_seconds**: DNS lookup duration (histogram)
- **http_client_tcp_duration_seconds**: TCP connection duration (histogram)
- **http_client_tls_duration_seconds**: TLS handshake duration (histogram)
- **http_client_ttfb_duration_seconds**: Time to First Byte (histogram)
- **http_client_retries_total**: Total retries (labels: endpoint, method)

### Backend Metrics

- **http_backend_requests_total**: Total requests received (labels: path, method, status_code)
- **http_backend_request_duration_seconds**: Processing duration (histogram)
- **http_backend_response_size_bytes**: Response size (histogram)
- **http_backend_dropped_connections_total**: Total dropped connections (labels: path, method)
- **http_backend_idled_connections_total**: Total idled connections (labels: path, method)
- **http_backend_idle_duration_seconds**: Idle connection duration (histogram)

### Metrics Example

```prometheus
# Backend requests by endpoint
http_backend_requests_total{method="GET",path="/health",status_code="200"} 150
http_backend_requests_total{method="GET",path="/unreliable",status_code="200"} 45

# Client request duration
http_client_request_duration_seconds_sum{endpoint="Health Check",method="GET"} 0.523
http_client_request_duration_seconds_count{endpoint="Health Check",method="GET"} 150

# Problematic connections
http_backend_dropped_connections_total{method="GET",path="/unreliable"} 15
http_backend_idled_connections_total{method="GET",path="/unreliable"} 10
```

## Project Structure

```
test-backend/
├── main.go          # Entry point and orchestration
├── config.go        # Configuration structures and parsing
├── client.go        # HTTP client implementation
├── backend.go       # HTTP server implementation
├── logger.go        # Logging system
├── metrics.go       # Prometheus metrics
├── config/
│   └── config.yaml  # Example configuration
├── manifests/       # OpenShift/Kubernetes manifests
│   ├── backend/     # Backend deployment manifests
│   ├── client/      # Client deployment manifests
│   └── sharding/    # Sharding configuration
├── go.mod           # Go dependencies
└── README.md        # This documentation
```

## Troubleshooting

### Client Cannot Connect to Backend

- Verify the port is available
- Check that the backend is running
- Review logs for connection errors

### Want More Details in Logs

```yaml
logging:
  level: debug
  verbose: true
```

## License

MIT
