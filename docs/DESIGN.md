# Design Philosophy

This document outlines the design philosophy for the Ocular project.
The system is engineered based on the core principles of simplicity, modularity, and reusability.

## Overview

The primary objective is to establish a dedicated code scanning orchestration system,
decoupled from continuous integration/continuous deployment (CI/CD) pipelines responsible for code deployment.
This separation allows the project to prioritize the quality of the information produced by the scans,
and analyze larger organization wide trends.

The system is architected to provide a flexible and configurable framework, enabling users to define:

* **Targets:** The specific codebases or artifacts to be analyzed.
* **Scanners:** The tools and processes utilized for scanning.
* **Result Storage:** The designated locations for storing and managing scan outputs.

## Key Design Principles

The following principles guide the architecture and development of this system:

1.  **Configurable Data Handling:** All aspects of data ingress, transformation, and egress are designed to be user-configurable, providing maximum flexibility in data pipeline management.
2.  **Declarative Static Configuration:** Static configurations are managed declaratively. This approach simplifies updates, promotes version control, and enhances the maintainability of system settings.
3.  **Containerized Dynamic Configuration:** Dynamic configurations and execution environments are managed via containerization. This ensures consistency, portability, and scalability of scanning operations.
