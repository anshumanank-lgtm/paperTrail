import signal
import grpc
from concurrent import futures

from proto import intelligence_pb2
from proto import intelligence_pb2_grpc

from engine import IntelligenceEngine


class IntelligenceService(
    intelligence_pb2_grpc.IntelligenceServiceServicer
):
    def __init__(self, engine):
        self.engine = engine

    def Health(self, request, context):
        return intelligence_pb2.HealthResponse(
            ready=True,
        )

    def Extract(self, request, context):
        result = self.engine.extract(request.text)

        return intelligence_pb2.ExtractResponse(
            title=result["title"],
            title_confidence=result["title_confidence"],
            document_type=result["document_type"],
            document_type_confidence=result["document_type_confidence"],
            entities=[
                intelligence_pb2.Entity(
                    type=entity["type"],
                    value=entity["value"],
                    confidence=entity["confidence"],
                )
                for entity in result["entities"]
            ],
        )

    def ExtractBatch(self, request, context):
        texts = [
            document.text
            for document in request.documents
        ]

        results = self.engine.extract_batch(texts)

        return intelligence_pb2.ExtractBatchResponse(
            documents=[
                intelligence_pb2.ExtractResponse(
                    title=result["title"],
                    title_confidence=result["title_confidence"],
                    document_type=result["document_type"],
                    document_type_confidence=result[
                        "document_type_confidence"
                    ],
                    entities=[
                        intelligence_pb2.Entity(
                            type=entity["type"],
                            value=entity["value"],
                            confidence=entity["confidence"],
                        )
                        for entity in result["entities"]
                    ],
                )
                for result in results
            ],
        )


def serve():
    engine = IntelligenceEngine()

    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=10)
    )

    intelligence_pb2_grpc.add_IntelligenceServiceServicer_to_server(
        IntelligenceService(engine),
        server,
    )

    server.add_insecure_port("127.0.0.1:50051")
    server.start()

    print("Intelligence gRPC server ready on 127.0.0.1:50051", flush=True)

    def shutdown(signum, frame):
        print(
            "Shutting down Intelligence gRPC server...",
            flush=True,
        )

        server.stop(grace=5).wait()

        print(
            "Intelligence gRPC server stopped",
            flush=True,
        )

    signal.signal(signal.SIGTERM, shutdown)
    signal.signal(signal.SIGINT, shutdown)

    server.wait_for_termination()


if __name__ == "__main__":
    serve()