#![no_std]
use soroban_sdk::{contract, contractimpl, Env, Val};

#[contract]
pub struct Contract;

#[contractimpl]
impl Contract {
    pub fn set(e: Env, key: Val, val: Val) {
        e.storage().persistent().set(&key, &val)
    }

    pub fn remove(e: Env, key: Val) {
        e.storage().persistent().remove(&key)
    }
}