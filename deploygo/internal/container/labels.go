package container

import "sync"

type containerLabelRegistry struct {
	mu     sync.Mutex
	labels map[string]string
}

func newContainerLabelRegistry() containerLabelRegistry {
	return containerLabelRegistry{
		labels: make(map[string]string),
	}
}

func (r *containerLabelRegistry) set(id, label string) {
	if r == nil || id == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.labels == nil {
		r.labels = make(map[string]string)
	}
	r.labels[id] = label
}

func (r *containerLabelRegistry) get(id string) string {
	if r == nil || id == "" {
		return ""
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	return r.labels[id]
}

func (r *containerLabelRegistry) remove(id string) {
	if r == nil || id == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.labels, id)
}
