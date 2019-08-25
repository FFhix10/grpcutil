// Code generated by protoc-gen-tstypes. DO NOT EDIT.

declare namespace nested {

    export enum Notification_Type {
        UNSPECIFIED = 0,
        TEXT = 1,
        VIDEO = 2,
        AUDIO = 3,
    }
    export interface Notification {
        message_type?: Notification_Type;
        content?: string;
    }

    export enum Tweet_Type {
        UNSPECIFIED = 0,
        ORIGINAL = 1,
        RETWEET = 2,
    }
    export interface Tweet {
        tweet_type?: Tweet_Type;
        content?: string;
    }

    export interface A_B {
        id?: string;
    }

    export interface A {
        id?: string;
        b?: A_B;
    }

}
