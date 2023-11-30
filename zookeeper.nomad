job "zookeeper" {
  datacenters = ["dc1"]
  type        = "service"
  namespace   = "kafka"

  group "zookeeper" {
    network {
      mode = "bridge"
    }

    service {
      connect {
        sidecar_service {}
        sidecar_task {
          resources {
            memory = 50
            cpu    = 50
          }
        }
      }
      port = "2181"
      name = "zookeeper"
    }

    // volume "zookeeper" {
    //   type      = "host"
    //   read_only = false
    //   source    = "zookeeper"
    // }
    
    task "zookeeper" {

      driver = "docker"
      config {
        image = "bitnami/zookeeper:3.8.0"
      }
      
      env {
        ZOOKEEPER_CLIENT_PORT = 2181
        ALLOW_ANONYMOUS_LOGIN = "yes"
        ZOO_ENABLE_AUTH       = "no"
      }

      // volume_mount {
      //   volume      = "zookeeper"
      //   destination = "/bitnami/zookeeper"
      //   read_only   = false
      // }

      resources {
        memory = 1000
        cpu    = 300
      }
    }
  }
}
