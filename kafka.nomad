job "kafka" {
  datacenters = ["dc1"]
  type        = "service"
  namespace   = "kafka"

  group "kafka" {
    network {
      mode = "bridge"
    }

    service {
      connect {
        sidecar_service {
          proxy {
            upstreams {
              destination_name = "zookeeper"
              local_bind_port  = 2181
            }
          }
        }
      }
      name = "kafka"
      port = "9092"
    }

    // volume "kafka" {
    //   type            = "host"
    //   read_only       = false
    //   source          = "kafka"
    // }

    task "kafka" {
      driver = "docker"
      config {
        image = "bitnami/kafka:3.1.1"
      }

      resources {
        memory     = 2000
        cpu        = 300
      }

      // volume_mount {
      //   volume      = "kafka"
      //   destination = "/bitnami/kafka"
      //   read_only   = false
      // }

      env {
        KAFKA_BROKER_ID                          = 1
        KAFKA_ZOOKEEPER_CONNECT                  = "${NOMAD_UPSTREAM_ADDR_zookeeper}"
        KAFKA_INTER_BROKER_LISTENER_NAME         = "INTERNAL"
        KAFKA_CFG_LISTENERS                      = "INTERNAL://:9093,CLIENT://:9092,EXTERNAL://:9094"
        KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP = "INTERNAL:PLAINTEXT,CLIENT:PLAINTEXT,EXTERNAL:PLAINTEXT"
        KAFKA_CFG_ADVERTISED_LISTENERS           = "INTERNAL://:9093,CLIENT://127.0.0.1:9092,EXTERNAL://${attr.unique.network.ip-address}:9094"
        ALLOW_PLAINTEXT_LISTENER                 = "yes"
      }
    }
  }
  group "kafka-exporter" {
    network {
      mode = "bridge"
      port "metrics" {
        to = -1
      }
    }
    service {
      connect {
        sidecar_service {
          proxy {
            upstreams {
              destination_name = "kafka"
              local_bind_port  = 9092
            }
            expose {
              path {
                path            = "/metrics"
                protocol        = "http"
                local_path_port = 9308
                listener_port   = "metrics"
              }
            }
          }
        }
      }
      name = "kafka-exporter"
      port = "metrics"
      meta {
        addr        = "${NOMAD_HOST_ADDR_metrics}"
        serviceName = "kafka-exporter"
      }
    }
    task "exporter" {
      driver = "docker"
      config {
        image   = "bitnami/kafka-exporter:1.3.1-debian-10-r14"
        args    = ["--kafka.server=${NOMAD_UPSTREAM_ADDR_kafka}", "--web.listen-address=:9308"]
      }
      resources {
        memory = 100
        cpu    = 100
      }
    }
  }
}