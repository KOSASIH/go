#![no_std]
use soroban_sdk::{contract, contractimpl, symbol_short, Address, Env, token, Symbol};

#[contract]
pub struct Contract;

const KEY: Symbol = symbol_short!("key");

#[contractimpl]
impl Contract {
    pub fn __constructor(env: Env, sender: Address, token_contract: Address, amount: i128) {
        let client = token::Client::new(&env, &token_contract);
        client.transfer(&sender, &env.current_contract_address(), &amount);
        env.storage().persistent().set(&KEY, &1_u32);
        env.storage().instance().set(&KEY, &2_u32);
        env.storage().temporary().set(&KEY, &3_u32);
    }
}
