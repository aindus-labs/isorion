job "webapp" {
  datacenters = ["dc1"]
  type        = "service"

  group "api" {
    count = 5

    network {
      port "p0" {
        static = 80
        to     = 8080
      }
    }

    task "api" {
      driver = "docker"

      config {
        image = "nginx:1.25"
        ports = ["p0"]
      }

      env {
        APP_ENV = "production"
      }
    }
  }

  group "db" {
    count = 1

    task "db" {
      driver = "docker"

      config {
        image = "postgres:16"
      }
    }
  }
}
