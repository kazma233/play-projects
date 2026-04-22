use std::collections::HashMap;
use std::sync::{Arc, Mutex};

use anyhow::{anyhow, Result};

use crate::{SessionFileEntry, SourceApp};

#[derive(Clone, Debug, Default)]
pub(crate) struct SessionFileCatalog {
    pub(crate) entries: Vec<SessionFileEntry>,
}

#[derive(Clone, Default)]
pub(crate) struct SessionIndexState {
    catalogs: Arc<Mutex<HashMap<SourceApp, SessionFileCatalog>>>,
}

impl SessionIndexState {
    pub(crate) fn catalog(&self, source_app: SourceApp) -> Result<Option<Vec<SessionFileEntry>>> {
        Ok(self
            .catalogs
            .lock()
            .map_err(|_| anyhow!("Session index cache lock was poisoned"))?
            .get(&source_app)
            .map(|catalog| catalog.entries.clone()))
    }

    pub(crate) fn store_catalog(
        &self,
        source_app: SourceApp,
        catalog: SessionFileCatalog,
    ) -> Result<()> {
        self.catalogs
            .lock()
            .map_err(|_| anyhow!("Session index cache lock was poisoned"))?
            .insert(source_app, catalog);
        Ok(())
    }

    pub(crate) fn clear(&self) -> Result<()> {
        self.catalogs
            .lock()
            .map_err(|_| anyhow!("Session index cache lock was poisoned"))?
            .clear();
        Ok(())
    }
}
