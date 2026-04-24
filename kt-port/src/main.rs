mod app;
mod ports;

fn main() -> Result<(), slint::PlatformError> {
    app::run()
}
