import MyIcon from '@/components/Icon';
import MySelect from '@/components/Select';
import { FormSourceInput, TemplateSourceType } from '@/types/app';
import {
  Box,
  Flex,
  FormControl,
  Input,
  Text,
  Checkbox,
  NumberInput,
  NumberInputField
} from '@chakra-ui/react';
import { useTranslation } from 'next-i18next';
import { useMemo } from 'react';
import { UseFormReturn } from 'react-hook-form';
import { evaluateExpression } from '@/utils/json-yaml';
import { getTemplateValues } from '@/utils/template';

const Form = ({
  formSource,
  formHook
}: {
  formSource: TemplateSourceType;
  formHook: UseFormReturn;
}) => {
  if (!formHook) return null;
  const { t } = useTranslation();

  const isShowContent = useMemo(
    () => !!formSource?.source?.inputs?.length,
    [formSource?.source?.inputs?.length]
  );

  const {
    register,
    formState: { errors },
    setValue
  } = formHook;
  const { defaults, defaultInputs } = getTemplateValues(formSource);
  return (
    <Box flexGrow={1} id={'baseInfo'} minH={'200px'}>
      {isShowContent ? (
        <Box py={'24px'}>
          {formSource?.source?.inputs
            ?.filter(
              (item) =>
                item.if === undefined ||
                item.if?.length === 0 ||
                evaluateExpression(item.if, {
                  ...formSource?.source,
                  inputs: {
                    ...defaultInputs,
                    ...formHook.getValues()
                  },
                  defaults: defaults
                })
            )
            .map((item: FormSourceInput, index: number) => {
              const commonInputProps = {
                maxW: '500px',
                ml: '20px',
                defaultValue: item?.default,
                placeholder: item?.description
              };

              const renderInput = () => {
                switch (item.type) {
                  case 'choice':
                    if (!item.options) {
                      console.error(`${item.key} options is required`);
                      return null;
                    }
                    return (
                      <MySelect
                        w={'100%'}
                        bg={'transparent'}
                        borderRadius={'2px'}
                        value={formHook.getValues(item.key) || item.default}
                        list={item.options?.map((option) => {
                          return {
                            value: option,
                            label: option
                          };
                        })}
                        onchange={(val: any) => {
                          setValue(item.key, val);
                        }}
                      />
                    );
                  case 'boolean':
                    const input = formHook.getValues(item.key)
                    return (
                      <Checkbox
                        isChecked={input !== undefined ? !!input : !!item.default}
                        onChange={(e) => {
                          setValue(item.key, e.target.checked);
                        }}
                      />
                    );
                  case 'number':
                    return (
                      <NumberInput
                        {...commonInputProps}
                        value={formHook.getValues(item.key) || item.default}
                        onChange={(valueString) => {
                          setValue(item.key, Number(valueString));
                        }}
                      >
                        <NumberInputField />
                      </NumberInput>
                    );
                  case 'string':
                  default:
                    return (
                      <Input
                        type={item?.type}
                        {...commonInputProps}
                        {...register(item?.key, {
                          required: item?.required
                        })}
                      />
                    );
                }
              };

              return (
                <FormControl key={item?.key} mb={7} isInvalid={!!errors.appName}>
                  <Flex alignItems={'center'} align="stretch">
                    <Flex
                      position={'relative'}
                      w="200px"
                      className="template-dynamic-label"
                      color={'#333'}
                      userSelect={'none'}
                    >
                      {item?.label}
                      {item?.required && (
                        <Text ml="2px" color={'#E53E3E'}>
                          *
                        </Text>
                      )}
                    </Flex>
                    <Box ml={'17px'} w={'100%'}>
                      {renderInput()}
                    </Box>
                  </Flex>
                </FormControl>
              );
            })}
        </Box>
      ) : (
        <Flex
          justifyContent={'center'}
          alignItems="center"
          h={'100%'}
          w={'100%'}
          flexDirection="column"
        >
          <Flex
            border={'1px dashed #9CA2A8'}
            borderRadius="50%"
            w={'48px'}
            h={'48px'}
            justifyContent="center"
            alignItems={'center'}
          >
            <MyIcon color={'#7B838B'} name="empty"></MyIcon>
          </Flex>
          <Text mt={'12px'} fontSize={14} color={'#5A646E'}>
            {t('Not need to configure any parameters')}
          </Text>
        </Flex>
      )}
    </Box>
  );
};

export default Form;