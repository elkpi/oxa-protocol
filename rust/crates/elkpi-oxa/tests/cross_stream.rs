//! Runs the cross-stream golden vectors (vectors/cross/stream/) through all
//! nine ordered face pairs of the Rust implementations (spec/21 §9).

use std::path::Path;

/// Face-generic stream adapter over one face's decoder/encoder pair; mirrors
/// the per-crate `StreamVectorConverter` adapters in the face test suites.
macro_rules! stream_converter {
    ($name:ident, $face:literal, $crate_:ident) => {
        struct $name {
            config: $crate_::Config,
            decoder: $crate_::StreamDecoder,
            encoder: $crate_::StreamEncoder,
        }

        impl $name {
            fn new() -> Self {
                let config = $crate_::Config::default();
                let decoder = $crate_::StreamDecoder::new(&config);
                let encoder = $crate_::StreamEncoder::new(&config);
                $name {
                    config,
                    decoder,
                    encoder,
                }
            }
        }

        impl oxa_vectest::StreamConverter for $name {
            fn face(&self) -> &'static str {
                $face
            }

            fn decode_native_event(&mut self, event: &str) -> Result<Vec<oxa_ir::Event>, String> {
                self.decoder.feed_json(event).map_err(|err| err.to_string())
            }

            fn flush_decoder(&mut self) -> Result<Vec<oxa_ir::Event>, String> {
                self.decoder.flush().map_err(|err| err.to_string())
            }

            fn decoder_losses(&self) -> Vec<oxa_ir::Loss> {
                self.decoder.losses().to_vec()
            }

            fn apply_ir_event(
                &mut self,
                event: &oxa_ir::Event,
            ) -> Result<(Vec<serde_json::Value>, Vec<oxa_ir::Loss>), String> {
                let (events, losses) = self.encoder.apply(event).map_err(|err| err.to_string())?;
                let values = events
                    .into_iter()
                    .map(|e| serde_json::to_value(e).map_err(|err| err.to_string()))
                    .collect::<Result<Vec<_>, _>>()?;
                Ok((values, losses))
            }

            fn reset_stream_vector(&mut self) {
                self.decoder = $crate_::StreamDecoder::new(&self.config);
                self.encoder = $crate_::StreamEncoder::new(&self.config);
            }
        }
    };
}

stream_converter!(Anthropic, "anthropic", oxa_anthropic);
stream_converter!(ChatCompletions, "chatcompletions", oxa_chatcompletions);
stream_converter!(Responses, "responses", oxa_responses);

fn assert_cross_stream(
    manifest_dir: &Path,
    source: &mut dyn oxa_vectest::StreamConverter,
    target: &mut dyn oxa_vectest::StreamConverter,
    expected: usize,
) {
    match oxa_vectest::run_cross_stream_in(manifest_dir, source, target) {
        Ok(oxa_vectest::Outcome::Skipped) => panic!(
            "cross stream vectors unexpectedly skipped; integration tests must locate the monorepo vectors"
        ),
        Ok(oxa_vectest::Outcome::Ran(report)) => {
            assert_eq!(
                report.executed,
                expected,
                "unexpected cross stream vector count for {} -> {}",
                source.face(),
                target.face()
            );
            assert!(
                report.failures.is_empty(),
                "cross stream vector failures for {} -> {}:\n{:#?}",
                source.face(),
                target.face(),
                report.failures
            );
        }
        Err(err) => panic!(
            "cross stream harness error for {} -> {}: {err}",
            source.face(),
            target.face()
        ),
    }
}

#[test]
fn cross_stream_vectors() {
    let manifest_dir = Path::new(env!("CARGO_MANIFEST_DIR"));

    // Pair counts across the ten vectors: one multi-delta-text vector per
    // ordered pair, plus two focused Chat Completions -> Anthropic vectors
    // (interleaved tool-argument fragments, usage-only terminal).
    {
        let mut source = ChatCompletions::new();
        let mut target = Anthropic::new();
        assert_cross_stream(manifest_dir, &mut source, &mut target, 2);
    }

    let mut source = ChatCompletions::new();
    let mut target = ChatCompletions::new();
    assert_cross_stream(manifest_dir, &mut source, &mut target, 1);

    let mut source = ChatCompletions::new();
    let mut target = Responses::new();
    assert_cross_stream(manifest_dir, &mut source, &mut target, 1);

    let mut source = Anthropic::new();
    let mut target = ChatCompletions::new();
    assert_cross_stream(manifest_dir, &mut source, &mut target, 1);

    let mut source = Anthropic::new();
    let mut target = Anthropic::new();
    assert_cross_stream(manifest_dir, &mut source, &mut target, 1);

    let mut source = Anthropic::new();
    let mut target = Responses::new();
    assert_cross_stream(manifest_dir, &mut source, &mut target, 1);

    let mut source = Responses::new();
    let mut target = ChatCompletions::new();
    assert_cross_stream(manifest_dir, &mut source, &mut target, 1);

    let mut source = Responses::new();
    let mut target = Anthropic::new();
    assert_cross_stream(manifest_dir, &mut source, &mut target, 1);

    let mut source = Responses::new();
    let mut target = Responses::new();
    assert_cross_stream(manifest_dir, &mut source, &mut target, 1);
}
