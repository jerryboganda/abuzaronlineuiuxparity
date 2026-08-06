declare global {
  namespace App {
    interface Locals {
      tenantId?: string;
      branchId?: string;
      counterId?: string;
      operatorId?: string;
    }
  }
}

export {};
