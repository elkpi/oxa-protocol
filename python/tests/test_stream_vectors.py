"""Runs all shared golden vectors for streaming across all faces against the Python implementation."""

import unittest

from dataclasses import replace

from oxa.anthropic.messages import (
    StreamDecoder as AnthropicStreamDecoder,
)
from oxa.anthropic.messages import (
    StreamEncoder as AnthropicStreamEncoder,
)
from oxa.openai.chatcompletions import (
    StreamDecoder as ChatCompletionsStreamDecoder,
)
from oxa.openai.chatcompletions import (
    StreamEncoder as ChatCompletionsStreamEncoder,
)
from oxa.openai.responses import (
    StreamDecoder as ResponsesStreamDecoder,
)
from oxa.openai.responses import (
    StreamEncoder as ResponsesStreamEncoder,
)
from oxa.ir import Violation, load_event_stream
from oxa.vectest import (
    find_repo_root,
    load_vectors,
    run_cross_stream_vector,
    run_stream_vector,
)

FACES = {
    "chatcompletions": (
        lambda table: ChatCompletionsStreamDecoder(table=table),
        lambda table: ChatCompletionsStreamEncoder(table=table),
    ),
    "anthropic": (
        lambda table: AnthropicStreamDecoder(table=table),
        lambda table: AnthropicStreamEncoder(table=table),
    ),
    "responses": (
        lambda table: ResponsesStreamDecoder(table=table),
        lambda table: ResponsesStreamEncoder(table=table),
    ),
}


class StreamVectorsTest(unittest.TestCase):
    def test_all_stream_vectors(self) -> None:
        root = find_repo_root()
        if root is None:
            self.skipTest("repository root not found (running as standalone package)")
            return

        total_vectors = 0
        for face, (dec_factory, enc_factory) in FACES.items():
            vectors = load_vectors(root, face, "stream")
            self.assertGreater(len(vectors), 0, f"expected stream vectors for face {face}")
            total_vectors += len(vectors)

            for vector in vectors:
                with self.subTest(vector=vector.name):
                    run_stream_vector(
                        vector,
                        decoder_factory=dec_factory,
                        encoder_factory=enc_factory,
                    )

        self.assertEqual(total_vectors, 52, "expected 52 total stream vectors across all faces")

    def test_all_cross_stream_vectors(self) -> None:
        root = find_repo_root()
        if root is None:
            self.skipTest("repository root not found (running as standalone package)")
            return

        vectors = load_vectors(root, "cross", "stream")
        self.assertEqual(len(vectors), 10, "expected 10 cross stream vectors")

        seen_pairs: set[tuple[str, str]] = set()
        for vector in vectors:
            with self.subTest(vector=vector.name):
                self.assertEqual(
                    vector.conversion,
                    "protocol-to-protocol",
                    f"unexpected conversion for {vector.name}",
                )
                self.assertIn(vector.source.protocol, FACES, vector.name)
                self.assertIn(vector.target.protocol, FACES, vector.name)
                run_cross_stream_vector(
                    vector,
                    source_decoder_factory=FACES[vector.source.protocol][0],
                    target_encoder_factory=FACES[vector.target.protocol][1],
                )
                seen_pairs.add((vector.source.protocol, vector.target.protocol))

        expected_pairs = {(s, t) for s in FACES for t in FACES}
        self.assertEqual(
            seen_pairs,
            expected_pairs,
            "cross stream vectors must cover all nine ordered face pairs",
        )

    def test_cross_stream_rejects_illegal_decoder_output(self) -> None:
        """The target encoder must not mask an illegal decoder stream: a
        decoder that emits a tool_use block whose input text never appears
        as InputJSONDelta fragments violates the strict stream grammar even
        though an encoder could synthesize the missing fragment."""
        root = find_repo_root()
        if root is None:
            self.skipTest("repository root not found (running as standalone package)")
            return

        source_vector = next(
            v for v in load_vectors(root, "anthropic", "stream") if v.name.endswith("m7-tool-use-from-ir")
        )
        events = [
            e
            for e in load_event_stream(source_vector.input).events
            if type(e).__name__ != "ContentBlockDelta"
        ]

        encoder = AnthropicStreamEncoder()
        wire_events: list[dict] = []
        for ev in events:
            produced, _ = encoder.apply(ev)
            wire_events.extend(produced)

        class IllegalDecoder:
            def feed(self, ev, raw_str):
                return events

            def flush(self):
                return []

            def losses(self):
                return []

        probe = replace(
            source_vector,
            conversion="protocol-to-protocol",
            input={"events": [{}]},
            input_raw='{"events":[{}]}',
            expected_output={"events": wire_events},
            expected_losses=[],
        )
        with self.assertRaises(Violation):
            run_cross_stream_vector(
                probe,
                source_decoder_factory=lambda table: IllegalDecoder(),
                target_encoder_factory=lambda table: AnthropicStreamEncoder(),
            )


if __name__ == "__main__":
    unittest.main()
