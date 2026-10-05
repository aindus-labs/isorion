package emit

import (
	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
)

// podmanComposeEmitter emits a podman-compose file — same compose schema,
// different artifact name (podman-compose reads docker-compose.yml too, but
// emitting podman-compose.yml keeps parallel stacks on one checkout).
type podmanComposeEmitter struct{}

func init() { Register(podmanComposeEmitter{}) }

func (podmanComposeEmitter) Name() string { return "podman-compose" }
func (podmanComposeEmitter) Description() string {
	return "Podman Compose file (podman-compose.yml)"
}

func (podmanComposeEmitter) Emit(doc *lang.Document, infra *model.Infra) ([]File, error) {
	return emitCompose(doc, infra, "podman-compose.yml", "podman-compose")
}
