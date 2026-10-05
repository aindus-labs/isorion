terraform {
  required_providers {
    docker = {
      source  = "kreuzwerker/docker"
      version = "~> 3.0"
    }
  }
}

provider "docker" {}

resource "docker_network" "front" {
  name = "webapp_front"
}

resource "docker_container" "api" {
  name  = "api"
  image = "nginx:1.25"
  ports {
    internal = 8080
    external = 80
  }
  env = ["APP_ENV=production"]
  depends_on = [docker_container.db]
}

resource "docker_container" "db" {
  name  = "db"
  image = "postgres:16"
}
