<?php

declare(strict_types=1);

namespace VexPay\Exception;

/**
 * Thrown when the VexPay API returns a non-2xx response.
 */
class ApiException extends VexPayException
{
    /**
     * @param int         $statusCode HTTP status code returned by the API.
     * @param string|null $errorBody  The raw response body, if any.
     */
    public function __construct(
        string $message,
        public readonly int $statusCode = 0,
        public readonly ?string $errorBody = null,
    ) {
        parent::__construct($message);
    }
}
