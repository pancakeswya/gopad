
declare global {
    namespace ot {
        interface Sequence {
            get Ops(): any[];

            set Ops(ops: any[]);

            get BaseLen(): number;

            set BaseLen(val: number);

            get TargetLen(): number;

            set TargetLen(val: number);

            Delete(n: number): void;

            Insert(str: string): void;

            Retain(n: number): void;

            Apply(s: string): string;

            IsNoop(): boolean;

            Compose(other: Sequence): Sequence;

            Transform(other: Sequence): [Sequence, Sequence];
            TransformIndex(position: number): number

            Invert(s: string): Sequence;

            ToString(): string;
        }

        function NewSequence(): Sequence
        function FromString(str: string): Sequence
    }
}

export default ot;

