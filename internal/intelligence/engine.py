import contextlib
import json
import sys

from gliner2 import GLiNER2


MODEL_NAME = "fastino/gliner2-base-v1"


DOCUMENT_TYPES = {
    "invoice": "A document requesting payment for goods or services.",
    "receipt": "Proof that a payment or transaction has been completed.",
    "bill": "A document requesting payment for a service or recurring charge.",
    "statement": "A financial or account statement showing transactions, balances, or activity.",
    "contract": "A legal agreement between two or more parties containing terms and obligations.",
    "insurance_policy": "An insurance document describing insurance coverage or policy details.",
    "ticket": "A travel, transportation, event, or admission ticket.",
    "identity": "An identity or identification document.",
    "payslip": "A salary or payroll document showing earnings and deductions.",
    "tax": "A tax-related document involving filing, assessment, or payment.",
    "warranty": "A warranty or guarantee document for a product or service.",
    "other": "A document that does not fit the supported document types.",
}


ENTITY_TYPES = {
    # Tier 1 — Always extract
    "person": "A person's name.",
    "organisation": "A company, institution, bank, insurer, government body, or other organisation.",
    "location": "A geographic location or place.",
    "date": "A date or date expression appearing in the document.",
    "time": "A time or time expression appearing in the document.",
    "money": "A monetary amount appearing in the document.",
    "percent": "A percentage or percentage expression appearing in the document.",
    "quantity": "A quantity expressed with a number and unit.",
    "email": "An email address.",
    "phone_number": "A telephone or mobile phone number.",
    "url": "A web URL.",
    "product": "A named product, software product, or commercial offering.",
    "event": "A named event, conference, meeting, ceremony, or other occurrence.",
    "vehicle": "A vehicle such as a car, motorcycle, truck, aircraft, or other identifiable vehicle.",

    # Tier 2 — Extract if present
    "address": "A physical postal or street address.",
    "id_number": "A document, reference, identification, employee, account, or other identifying number.",
    "job_title": "A person's professional or occupational title.",
    "law": "A named law or legislation.",
    "regulation": "A named regulation, regulatory rule, or regulatory standard.",
    "language": "A human language.",
    "social_handle": "A social media username or handle.",

    # Tier 3 — Extract the document title if present
    "document_title": "The title or heading that identifies the document.",
}


print("Loading GLiNER2...", file=sys.stderr)

with contextlib.redirect_stdout(sys.stderr):
    extractor = GLiNER2.from_pretrained(MODEL_NAME)


schema = (
    extractor.create_schema()
    .entities(ENTITY_TYPES)
    .classification(
        "document_type",
        DOCUMENT_TYPES,
    )
)


print("GLiNER2 intelligence engine ready", file=sys.stderr)


def process(content):
    text = content.get("text", "")

    if not text.strip():
        return {
            "title": "",
            "title_confidence": 0.0,
            "document_type": "unknown",
            "document_type_confidence": 0.0,
            "entities": [],
        }

    result = extractor.extract(
        text,
        schema,
        include_confidence=True,
    )

    title = ""
    title_confidence = 0.0
    entities = []

    for label, values in result.get("entities", {}).items():
        for value in values:
            if isinstance(value, dict):
                entity_value = value.get("text", "")
                confidence = value.get("confidence", 0.0)
            else:
                entity_value = value
                confidence = 1.0

            if label == "document_title":
                if entity_value:
                    title = entity_value
                    title_confidence = confidence
                continue

            entities.append({
                "type": label,
                "value": entity_value,
                "confidence": confidence,
            })

    document_type = result.get("document_type", "unknown")
    document_type_confidence = 0.0

    if isinstance(document_type, dict):
        document_type_confidence = document_type.get("confidence", 0.0)
        document_type = document_type.get("label", "unknown")

    return {
        "title": title,
        "title_confidence": title_confidence,
        "document_type": document_type,
        "document_type_confidence": document_type_confidence,
        "entities": entities,
    }


def main():
    for line in sys.stdin:
        line = line.strip()

        if not line:
            continue

        try:
            content = json.loads(line)
            result = process(content)
            print(json.dumps(result), flush=True)

        except Exception as exc:
            print(
                json.dumps({
                    "error": str(exc),
                }),
                flush=True,
            )


if __name__ == "__main__":
    main()