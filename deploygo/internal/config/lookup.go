package config

func (cfg *Config) FindBuild(name string) *StageConfig {
	if cfg == nil {
		return nil
	}

	for i := range cfg.Builds {
		if cfg.Builds[i].Name == name {
			return &cfg.Builds[i]
		}
	}

	return nil
}

func (cfg *Config) FindDeployStep(name string) *DeploymentStep {
	if cfg == nil {
		return nil
	}

	for i := range cfg.Deploys {
		if cfg.Deploys[i].Name == name {
			return &cfg.Deploys[i]
		}
	}

	return nil
}

func (cfg *Config) FindServer(name string) *ServerConfig {
	if cfg == nil {
		return nil
	}

	server, ok := cfg.Servers[name]
	if !ok {
		return nil
	}

	return &server
}
