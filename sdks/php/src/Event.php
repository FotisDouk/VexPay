<?php

declare(strict_types=1);

namespace VexPay;

/**
 * A webhook event delivered by VexPay, e.g. "invoice.paid".
 */
final class Event
{
    /**
     * @param array<string,mixed> $raw
     */
    public function __construct(
        public readonly string $id,
        public readonly string $type,
        public readonly ?string $created,
        public readonly Invoice $invoice,
        public readonly array $raw,
    ) {
    }

    /**
     * @param array<string,mixed> $data
     */
    public static function fromArray(array $data): self
    {
        return new self(
            id: (string) ($data['id'] ?? ''),
            type: (string) ($data['type'] ?? ''),
            created: isset($data['created']) ? (string) $data['created'] : null,
            invoice: Invoice::fromArray($data['data'] ?? []),
            raw: $data,
        );
    }
}
