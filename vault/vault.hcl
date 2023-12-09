ui           = true
log_level    = "trace"
cluster_addr = "http://127.0.0.1:8201"
api_addr     = "http://127.0.0.1:8200"
cluster_name = "%s"

storage "consul" {
  address = "http://localhost:8500"
  path    = "vault/"
}

listener "tcp" {
    address = "0.0.0.0:8200"
    cluster_address  = "0.0.0.0:8201"
    tls_disable = 1
}

max_lease_ttl = "9000h"
default_lease_ttl = "10h"
ui = true