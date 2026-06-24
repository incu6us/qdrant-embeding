Feature: Ingest markdown documents into the vector store
  As a developer building a retrieval pipeline
  I want markdown files embedded and stored idempotently
  So that re-running ingestion keeps the vector store consistent

  Background:
    Given an embedder producing 384-dimensional vectors
    And an empty vector store

  Scenario: Ingesting a directory of markdown documents
    Given a source containing 3 markdown documents
    When I run the ingestion
    Then 3 documents should be reported as ingested
    And the vector store should contain 3 embedded documents
    And the vector store should be prepared with dimension 384

  Scenario: Re-ingesting the same documents does not create duplicates
    Given a source containing 2 markdown documents
    When I run the ingestion
    And I run the ingestion again
    Then the vector store should contain 2 embedded documents
